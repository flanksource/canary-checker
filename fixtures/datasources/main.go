package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/utils"
	"github.com/ncw/swift"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"

	amqp "github.com/rabbitmq/amqp091-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func prepareS3E2E(fixture S3Fixture) error {
	client, err := getS3Client()
	if err != nil {
		return errors.Wrap(err, "failed to get s3 client")
	}

	for _, bucket := range fixture.CreateBuckets {
		req := &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		}
		if _, err := client.CreateBucket(req); err != nil {
			return errors.Wrapf(err, "failed to create bucket %s", bucket)
		}
	}

	for _, file := range fixture.Files {
		body := utils.RandomString(int(file.Size))
		_, err = client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(file.Bucket),
			Key:         aws.String(file.Filename),
			Body:        bytes.NewReader([]byte(body)),
			ContentType: aws.String(file.ContentType),
			Metadata: map[string]*string{
				"Last-Modified": aws.String(swift.TimeToFloatString(time.Now().Add(-1 * file.Age))),
			},
		})
		if err != nil {
			return errors.Wrapf(err, "failed to put object %s to bucket %s", file.Filename, file.Bucket)
		}
	}
	return nil
}

func cleanupS3E2E(fixture S3Fixture) {
	client, err := getS3Client()
	if err != nil {
		logger.Errorf("failed to create s3 client: %v", err)
		return
	}

	for _, file := range fixture.Files {
		_, err := client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(file.Bucket),
			Key:    aws.String(file.Filename),
		})
		if err != nil {
			logger.Errorf("failed to delete object %s in bucket %s: %v", file.Filename, file.Bucket, err)
		}
	}

	for _, bucket := range fixture.CreateBuckets {
		if _, err := client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			logger.Errorf("failed to delete bucket %s: %v", bucket, err)
		}
	}
}

type S3Config struct {
	AccessKey string
	SecretKey string
	Region    string
	Endpoint  string
}

func getS3Client() (*s3.S3, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	s3Cfg := getS3Credentials()
	cfg := aws.NewConfig().
		WithRegion(s3Cfg.Region).
		WithEndpoint(s3Cfg.Endpoint).
		WithCredentials(
			credentials.NewStaticCredentials(s3Cfg.AccessKey, s3Cfg.SecretKey, ""),
		).
		WithHTTPClient(&http.Client{Transport: tr})
	ssn, err := session.NewSession(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create s3 session")
	}
	client := s3.New(ssn)
	client.Config.S3ForcePathStyle = aws.Bool(true)
	return client, nil
}

func getS3Credentials() S3Config {
	cfg := S3Config{
		AccessKey: getEnvOrDefault("S3_ACCESS_KEY", "minio"),
		SecretKey: getEnvOrDefault("S3_SECRET_KEY", "minio123"),
		Region:    getEnvOrDefault("S3_REGION", "minio"),
		Endpoint:  getEnvOrDefault("S3_ENDPOINT", "https://minio.127.0.0.1.nip.io"),
	}
	return cfg
}

type S3Fixture struct {
	CreateBuckets []string
	Files         []S3FixtureFile
}

type S3FixtureFile struct {
	Bucket      string
	Filename    string
	Size        int64
	Age         time.Duration
	ContentType string
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

func forward(
	config *restclient.Config, name, publish string,
) (uint16, func() error, error) {
	config.GroupVersion = &schema.GroupVersion{Group: "api", Version: "v1"}
	codecs := serializer.NewCodecFactory(runtime.NewScheme())
	s := serializer.WithoutConversionCodecFactory{CodecFactory: codecs}
	config.NegotiatedSerializer = s
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return 0, nil, err
	}
	client, err := restclient.RESTClientFor(config)
	if err != nil {
		return 0, nil, err
	}
	req := client.Post().
		Resource("pods").
		Namespace("default").
		Name(name).
		SubResource("portforward")
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	errChan := make(chan error)
	doneFunc := func() error {
		close(stopChan)
		return <-errChan
	}
	pf, err := portforward.New(
		dialer, []string{publish}, stopChan, readyChan, os.Stdout, os.Stderr,
	)
	if err != nil {
		return 0, nil, err
	}
	go func() {
		errChan <- pf.ForwardPorts()
		close(errChan)
	}()
	select {
	case err := <-errChan:
		close(stopChan)
		return 0, nil, err
	case <-pf.Ready:
		ports, err := pf.GetPorts()
		if err != nil {
			return 0, nil, err
		}
		return ports[0].Local, doneFunc, nil
	case <-time.After(10 * time.Second):
		return 0, nil, fmt.Errorf("Timed out trying to forward %s on %s", name, publish)
	}
}

func getPodName(ctx context.Context, config *restclient.Config, selector string) (string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	pods, err := clientset.CoreV1().Pods("default").
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", err
	}
	if len(pods.Items) != 1 {
		return "", fmt.Errorf("Expected a pod, got none")
	}
	return pods.Items[0].Name, nil
}

func prepOneAMQP(host, xname, xtype, qname, bkey, skey string) error {
	// Get secrets
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	ctx := context.TODO()
	sec, err := clientset.CoreV1().Secrets("default").
		Get(ctx, "amqp-fixture-rabbitmq-admin", metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Forward
	podName, err := getPodName(ctx, config, "app.kubernetes.io/name=amqp-fixture")
	if err != nil {
		return err
	}
	port, doneFunc, err := forward(config, podName, ":5672")
	if err != nil {
		return err
	}
	defer doneFunc() // could wrap this to update named rv if non-nil
	// Open
	addr := fmt.Sprintf(
		"amqp://%s:%s@%s:%d", sec.Data["username"], sec.Data["password"], host, port,
	)
	conn, err := amqp.Dial(addr)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	// Create exchange
	if err := ch.ExchangeDeclare(xname, xtype, false, false, false, false, nil); err != nil {
		return err
	}
	// Create queue
	if _, err := ch.QueueDeclare(qname, false, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(qname, bkey, xname, false, nil); err != nil {
		return err
	}
	// Publish
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p := amqp.Publishing{ContentType: "text/plain", Body: []byte("e2e:" + bkey)}
	if err := ch.PublishWithContext(ctx, xname, skey, false, false, p); err != nil {
		return err
	}
	return nil
}

func prepareAMQPE2E() error {
	// Skip testPeekDirectFail (canary will create and bind)
	if err := prepOneAMQP(
		"localhost", "canary.test.peek.direct", "direct",
		"testPeekDirectPass", "direct.pass", "direct.pass",
	); err != nil {
		return err
	}
	// Create fail first
	if err := prepOneAMQP(
		"localhost", "canary.test.peek.topic", "topic",
		"testPeekTopicFail", "#.fail.#", "topic.pass.foo",
	); err != nil {
		return err
	}
	if err := prepOneAMQP(
		"localhost", "canary.test.peek.topic", "topic",
		"testPeekTopicPass", "#.pass.#", "topic.pass.foo",
	); err != nil {
		return err
	}
	return nil
}

func main() {
	logger.Infof("Setting up")
	if err := prepareS3E2E(s3Fixtures); err != nil {
		logger.Errorf("error setting up %v", err)
	}
	if err := prepareAMQPE2E(); err != nil {
		logger.Errorf("error preparing AMQP E2E %v", err)
	}
}

var (
	s3Fixtures = S3Fixture{

		CreateBuckets: []string{
			"tests-e2e-1",
			"tests-e2e-2",
		},
		Files: []S3FixtureFile{
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/pg/backups/date1/backup.zip",
				Size:        50,
				Age:         30 * 24 * time.Hour, // 30 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/pg/backups/date2/backup.zip",
				Size:        50,
				Age:         7 * 24 * time.Hour, // 7 days
				ContentType: "application/zip",
			},
			{
				Bucket:      "tests-e2e-1",
				Filename:    "/mysql/backups/date1/mysql.zip",
				Size:        30,
				Age:         7*24*time.Hour - 10*time.Minute, // 30 days
				ContentType: "application/zip",
			},
		},
	}
)

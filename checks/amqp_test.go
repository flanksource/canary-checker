package checks

// FIXME: delete this file after covering everything via e2e.

import (
	"bytes"
	gocontext "context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	amqp "github.com/rabbitmq/amqp091-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// These should probably be set via env var or flags
var (
	instanceSecretUser string
	instanceLabel      string
	serviceHost        string
	localhost          = "localhost"
	namespace          = "default"
	kubeconfig         string
	overrideURL        string
)

// For now, the env vars
//
// - RABBITMQ_CLUSTER_NAME=amqp-fixture
// - RABBITMQ_OPERATOR_LEGACY=1
//
// determine these params (whose defaults are "hello-world" and false)
func populateRabbitMQVars(name string, wantOld bool) {
	instanceLabel = fmt.Sprintf("app.kubernetes.io/name=%s", name)
	if wantOld {
		instanceSecretUser = fmt.Sprintf("%s-rabbitmq-admin", name)
		serviceHost = fmt.Sprintf("%s-rabbitmq-client.%s.svc", name, namespace)
		return
	}
	instanceSecretUser = fmt.Sprintf("%s-%s-user", name, namespace)
	serviceHost = fmt.Sprintf("%s.%s.svc", name, namespace)
}

func forward(
	config *rest.Config, name, publish string,
) (uint16, func() error, error) {
	config.GroupVersion = &schema.GroupVersion{Group: "api", Version: "v1"}
	codecs := serializer.NewCodecFactory(runtime.NewScheme())
	s := serializer.WithoutConversionCodecFactory{CodecFactory: codecs}
	config.NegotiatedSerializer = s
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return 0, nil, err
	}
	client, err := rest.RESTClientFor(config)
	if err != nil {
		return 0, nil, err
	}
	req := client.Post().
		Resource("pods").
		Namespace(namespace).
		Name(name).
		SubResource("portforward")
	errorChan := make(chan error, 1)
	stopChan := make(chan struct{})
	doneFunc := func() error {
		close(stopChan)
		return <-errorChan
	}
	stdout := &bytes.Buffer{}
	dialer := spdy.NewDialer(
		upgrader, &http.Client{Transport: transport}, "POST", req.URL(),
	)
	pf, err := portforward.New(
		dialer, []string{publish}, stopChan, make(chan struct{}), stdout, os.Stderr,
	)
	if err != nil {
		err = fmt.Errorf("%s: %w", stdout.String(), err)
		return 0, nil, err
	}
	go func() {
		errorChan <- pf.ForwardPorts()
		close(errorChan)
	}()
	select {
	case err := <-errorChan:
		close(stopChan)
		return 0, nil, err
	case <-pf.Ready:
		ports, err := pf.GetPorts()
		if err != nil {
			return 0, nil, err
		}
		return ports[0].Local, doneFunc, nil
	case <-time.After(10 * time.Second):
		return 0, nil, fmt.Errorf("Timed out forwarding %s on %s", name, publish)
	}
}

func getPodName(
	ctx gocontext.Context, config *rest.Config, selector string,
) (string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	pods, err := clientset.CoreV1().Pods(namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", err
	}
	if len(pods.Items) != 1 {
		return "", fmt.Errorf("Expected a pod, got none")
	}
	return pods.Items[0].Name, nil
}

// FIXME: use flanksource kommons for client-go stuff.
func devineConnection(
	host string, config *rest.Config, inCluster bool,
) (string, func() error, error) {
	// Get secrets
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", nil, err
	}
	ctx := gocontext.TODO()
	sec, err := clientset.CoreV1().Secrets(namespace).
		Get(ctx, instanceSecretUser, metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}
	// Maybe forward
	podName, err := getPodName(ctx, config, instanceLabel)
	if err != nil {
		return "", nil, err
	}
	var port uint16 = 5672
	var doneFunc func() error
	// For inCluster, we could look up the port for appProtocol == amqp, but
	// it's always 5672 anyway
	if !inCluster {
		port, doneFunc, err = forward(config, podName, ":5672")
		if err != nil {
			return "", nil, err
		}
	}
	addr := fmt.Sprintf(
		"amqp://%s:%s@%s:%d",
		sec.Data["username"],
		sec.Data["password"],
		host,
		port,
	)
	return addr, doneFunc, nil
}

func getEndpoint() (string, func() error, error) {
	if overrideURL != "" {
		return overrideURL, nil, nil
	}
	var config *rest.Config
	var host string
	if kubeconfig != "" {
		c, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return "", nil, err
		}
		config, host = c, localhost
	} else {
		c, err := rest.InClusterConfig()
		if err != nil {
			return "", nil, err
		}
		config, host = c, serviceHost
	}
	return devineConnection(host, config, kubeconfig == "")
}

func deleteQueue(ac *amqpCheck, queue string) (int, error) {
	conn, ch, err := ac.open()
	if err != nil {
		return 0, err
	}
	n, err := ch.QueueDelete(queue, false, false, false)
	if err != nil {
		return 0, err
	}
	ch.Close()
	conn.Close()
	return n, nil
}

func deleteExchanges(ac *amqpCheck, exchanges ...string) error {
	conn, ch, err := ac.open()
	if err != nil {
		return err
	}
	for _, ex := range exchanges {
		err = ch.ExchangeDelete(ex, false, false)
		if err != nil {
			return err
		}
	}
	ch.Close()
	conn.Close()
	return nil
}

func init() {
	if o := os.Getenv("RABBITMQ_OVERRIDE_URL"); o != "" {
		overrideURL = o
	} else {
		if k := os.Getenv("KUBECONFIG"); k != "" {
			kubeconfig = k
			return
		}
		if h := homedir.HomeDir(); h != "" {
			k := filepath.Join(h, ".kube", "config")
			if _, err := os.Stat(k); err == nil {
				kubeconfig = k
			}
		}
	}
	name := "hello-world"
	if n := os.Getenv("RABBITMQ_CLUSTER_NAME"); n != "" {
		name = n
	}
	// FIXME maybe find actual version where syntax diverged
	var wantOld bool
	if o := os.Getenv("RABBITMQ_OPERATOR_LEGACY"); o != "" {
		if w, err := strconv.ParseBool(o); err != nil {
			wantOld = w
		}
	}
	populateRabbitMQVars(name, wantOld)
}

func TestAmqpPrepURL(t *testing.T) {
	// Creds added
	addr, err := amqpPrepURL("foo.bar.svc", "bob", "changeme")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "amqp://bob:changeme@foo.bar.svc/" {
		t.Fatal("Expected username in addr", addr)
	}
	// Superfluous port dropped (upstream)
	addr, err = amqpPrepURL("foo.bar.svc:5672", "bob", "changeme")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "amqp://bob:changeme@foo.bar.svc/" {
		t.Fatal("Expected username in addr", addr)
	}
	// Roundtrip sans creds
	addr, err = amqpPrepURL(addr, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "amqp://foo.bar.svc/" {
		t.Fatal("Expected username absent from addr", addr)
	}
	// Scheme adjusted for TLS
	addr, err = amqpPrepURL("foo.bar.svc:5671", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "amqps://foo.bar.svc/" {
		t.Fatal("Expected username in addr", addr)
	}
}

const headersSample = `
---
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: amqp-peek-headers-pass
spec:
  interval: 30
  amqp:
    - name: amqp-peek-headers-pass
      addr: amqp-fixture.default.svc
      description: "AMQP headers pass test"
      exchange:
        name: testPeek.headers
        type: headers
        durable: true
      queue:
        name: testPeekHeaders
      simulation:
        bootstrap: true
        headers:
          hello: world
      binding:
        args:
          hello: world
          foo: 42
          x-match: any
`

func TestDesRawTables(t *testing.T) {
	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, "TestDesRawTables.yaml")
	err := os.WriteFile(tmpfile, []byte(headersSample), 0600)
	if err != nil {
		t.Error(err)
	}
	canaries, err := pkg.ParseConfig(tmpfile, "")
	if err != nil {
		t.Error(err)
	}
	canary := canaries[0]
	check := canary.Spec.AMQP[0]
	ac := amqpCheck{AMQPCheck: check}
	if err := ac.desRawTables(); err != nil {
		t.Fatal(err)
	}
	if ac.args["foo"] != float64(42) {
		t.Errorf("expected 42, got: %T, %v", ac.args["foo"], ac.args["foo"])
	}
	if ac.headers["hello"] != "world" {
		t.Errorf("expected hello world, got: %v", ac.headers)
	}
}

func wrapDone(spit func(...interface{}), f func() error) {
	if f == nil {
		return
	}
	if err := f(); err != nil {
		spit(err)
	}
}

func TestAmqpPushPull(t *testing.T) {
	addr, done, err := getEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	defer wrapDone(t.Fatal, done)
	ctx := &context.Context{Context: gocontext.TODO()}
	// Default
	qu := v1.AMQPQueue{Name: "check"}
	ac := &amqpCheck{v1.AMQPCheck{Queue: qu}, addr, "", t.Name(), nil, nil, nil}
	if err := ac.validate(); err != nil {
		t.Fatal(err)
	}
	err = ac.pushPull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// ACK'd
	ac = &amqpCheck{
		v1.AMQPCheck{Queue: qu, Ack: true}, addr, "", t.Name(), nil, nil, nil,
	}
	if err := ac.validate(); err != nil {
		t.Fatal(err)
	}
	err = ac.pushPull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Delete
	n, err := deleteQueue(ac, "check")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("Wrong number of messages remaining: %d", n)
	}
}

func TestAmqpWitness(t *testing.T) {
	addr, done, err := getEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	defer wrapDone(t.Fatal, done)
	ctx := &context.Context{Context: gocontext.TODO()}
	// Blind multicast
	aC := v1.AMQPCheck{
		Exchange:   v1.AMQPExchange{Name: "canary.fanout", Type: "fanout"},
		Simulation: v1.AMQPSimulation{Witness: true},
	}
	ac := &amqpCheck{aC, addr, "", t.Name(), nil, nil, nil}
	if err := ac.validate(); err != nil {
		t.Fatal(err)
	}
	err = ac.witness(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Explicit targeting/routing
	ac.Exchange.Name, ac.Exchange.Type = "canary.direct", "direct"
	ac.Simulation.Key = "direct." + t.Name()
	ac.Binding.Key = ac.Simulation.Key
	ac = &amqpCheck{aC, addr, "", t.Name(), nil, nil, nil}
	if err := ac.validate(); err != nil {
		t.Fatal(err)
	}
	err = ac.witness(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Pattern matching
	ac.Exchange.Name, ac.Exchange.Type = "canary.topic", "topic"
	ac.Simulation.Key = "topic." + t.Name()
	ac.Binding.Key = "topic.*"
	ac = &amqpCheck{aC, addr, "", t.Name(), nil, nil, nil}
	if err := ac.validate(); err != nil {
		t.Fatal(err)
	}
	err = ac.witness(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Delete
	err = deleteExchanges(ac, "canary.direct", "canary.fanout", "canary.topic")
	if err != nil {
		t.Fatal(err)
	}
}

func runAMQPPeek(
	kind, xName, qName, bindKey, sendKey string,
	args, hdrs amqp.Table,
	bs bool,
) error {
	addr, done, err := getEndpoint()
	if err != nil {
		return err
	}
	if done != nil {
		defer func() { _ = done() }()
	}
	ctx := &context.Context{Context: gocontext.TODO()}
	aC := v1.AMQPCheck{
		Exchange:   v1.AMQPExchange{Name: xName, Type: kind},
		Queue:      v1.AMQPQueue{Name: qName},
		Simulation: v1.AMQPSimulation{Key: sendKey},
		Binding:    v1.AMQPBinding{Key: bindKey},
		Peek:       true,
	}
	ac := &amqpCheck{aC, addr, "", "body=" + qName, nil, hdrs, args}
	if err := ac.validate(); err != nil {
		return err
	}
	// Prepare
	if bs {
		ac.Simulation.Bootstrap = true
	} else {
		if err := ac.push(ctx); err != nil {
			return err
		}
		ac.Simulation.Key = ""
		ac.headers = nil
	}
	// Default
	if err := ac.pushPull(ctx); err != nil {
		return err
	}
	if ac.qStatus.Messages != 1 {
		return fmt.Errorf("Expected 1 message")
	}
	// Delete
	n, err := deleteQueue(ac, qName)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("Wrong number of messages remaining: %d", n)
	}
	err = deleteExchanges(ac, xName)
	if err != nil {
		return err
	}
	return nil
}

func TestAmqpPeekDirect(t *testing.T) {
	if err := runAMQPPeek(
		"direct", t.Name()+".normal", t.Name(), "foo", "foo", nil, nil, false,
	); err != nil {
		t.Fatal(err)
	}
	if err := runAMQPPeek(
		"direct", t.Name()+".bootstrap", t.Name(), "foo", "foo", nil, nil, true,
	); err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekTopic(t *testing.T) {
	if err := runAMQPPeek(
		"topic", t.Name()+".normal", t.Name(), "foo.*", "foo.bar", nil, nil, false,
	); err != nil {
		t.Fatal(err)
	}
	if err := runAMQPPeek(
		"topic", t.Name()+".bootstrap", t.Name(), "foo.*", "foo.bar", nil, nil, true,
	); err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekFanout(t *testing.T) {
	err := runAMQPPeek(
		"fanout", t.Name()+".normal", t.Name(), "", "", nil, nil, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = runAMQPPeek(
		"fanout", t.Name()+".bootstrap", t.Name(), "", "", nil, nil, true,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekHeaders(t *testing.T) {
	err := runAMQPPeek(
		"headers",
		t.Name()+".normal", t.Name(),
		"", "",
		map[string]interface{}{"hello": "world"},
		map[string]interface{}{"hello": "world", "meaning": 42, "x-match": "any"},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = runAMQPPeek(
		"headers",
		t.Name()+".bootstrap", t.Name(),
		"", "",
		map[string]interface{}{"hello": "world"},
		map[string]interface{}{"hello": "world", "meaning": 42, "x-match": "any"},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
}

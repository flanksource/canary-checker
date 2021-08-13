package checks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/timer"
	"github.com/hairyhenderson/gomplate/v3/base64"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"net/http"

	"github.com/flanksource/kommons"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	prometheusCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_ec2_total",
			Help: "Number of times the ec2checker has run",
		},
		[]string{"region"},
	)
	prometheusFailCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_ec2_failed",
			Help: "Number of times the ec2checker has failed",
		},
		[]string{"region"},
	)
	prometheusPassCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_ec2_passed",
			Help: "Number of times the ec2checker has passed",
		},
		[]string{"region"},
	)

	prometheusStartupTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_ec2_start_time",
			Help: "ec2 instance startup time",
		},
		[]string{"region"},
	)
	prometheusTerminateTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_ec2_terminate_time",
			Help: "ec2 instance termination time",
		},
		[]string{"region"},
	)
	prometheusResponseTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_ec2_response_time",
			Help: "ec2 instance http response time",
		},
		[]string{"region"},
	)
)

func init() {
	prometheus.MustRegister(prometheusCount, prometheusFailCount, prometheusPassCount, prometheusResponseTime, prometheusStartupTime, prometheusTerminateTime)
}

type EC2Checker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *EC2Checker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c EC2Checker) GetClient() *kommons.Client {
	return c.kommons
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *EC2Checker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	if canary.Spec.EC2.AMI != "" {
		results = append(results, c.Check(canary, canary.Spec.EC2))
	}
	return results
}

// Type: returns checker type
func (c *EC2Checker) Type() string {
	return "ec2"
}

func (c *EC2Checker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.EC2Check)
	prometheusCount.WithLabelValues(check.Region).Inc()
	namespace := canary.Namespace

	kommonsClient := c.GetClient()
	if kommonsClient == nil {
		return HandleFail(check, "Kommons client not configured for ec2 checker")
	}
	_, accessKey, err := kommonsClient.GetEnvValue(check.AccessKeyID, namespace)
	if err != nil {
		return HandleFail(check, fmt.Sprintf("Could not parse EC2 access key: %v", err))
	}
	_, secretKey, err := kommonsClient.GetEnvValue(check.SecretKey, namespace)
	if err != nil {
		return HandleFail(check, fmt.Sprintf("Could not parse EC2 secret key: %v", err))
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: check.SkipTLSVerify},
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion(check.Region),
		config.WithHTTPClient(&http.Client{Transport: tr}),
	)
	if err != nil {
		return HandleFail(check, fmt.Sprintf("failed to load AWS credentials: %v", err))
	}

	var ami *string
	if check.AMI == "" {
		ssmClient := ssm.NewFromConfig(cfg)
		arnLookupInput := &ssm.GetParameterInput{Name: aws.String("/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2")}
		arnLookupOutput, err := ssmClient.GetParameter(context.TODO(), arnLookupInput)
		if err != nil {
			return HandleFail(check, fmt.Sprintf("Could not look up amazon image arn: %v", err))
		}
		ami = arnLookupOutput.Parameter.Value
	} else {
		ami = aws.String(check.AMI)
	}

	client := ec2.NewFromConfig(cfg)
	idString := fmt.Sprintf("%v/%v/%v", canary.ClusterName, namespace, canary.Name)

	describeInput := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name: aws.String("tag:type"),
				Values: []string{
					"canary-checker",
				},
			},
			{
				Name: aws.String("tag:owner"),
				Values: []string{
					idString,
				},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"running",
					"pending",
				},
			},
		},
	}

	describeOutput, err := client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		return HandleFail(check, fmt.Sprintf("Could not perform prerun check: %v", err))
	}

	staleIds := []string{}
	for r := range describeOutput.Reservations {
		for i := range describeOutput.Reservations[r].Instances {
			staleIds = append(staleIds, *describeOutput.Reservations[r].Instances[i].InstanceId)
		}
	}
	if len(staleIds) > 0 {
		logger.Infof("Found %v stale ec2 instances, terminating...", len(staleIds))
		err = terminateInstances(client, staleIds, 300000)
		if err != nil {
			return HandleFail(check, fmt.Sprintf("Could not terminate stale instances: %s", err))
		}
	}
	userData, err := base64.Encode([]byte(check.UserData))
	if err != nil {
		return HandleFail(check, fmt.Sprintf("Error encoding userData: %s", err))
	}
	if check.SecurityGroup == "" {
		check.SecurityGroup = "default"
	}

	runInput := &ec2.RunInstancesInput{
		ImageId:      ami,
		InstanceType: types.InstanceTypeT3Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("type"),
						Value: aws.String("canary-checker"),
					},
					{
						Key:   aws.String("owner"),
						Value: aws.String(idString),
					},
				},
			},
		},
		UserData: aws.String(userData),
		SecurityGroups: []string{
			check.SecurityGroup,
		},
	}

	timeTracker := NewTimer()

	runOutput, err := client.RunInstances(context.TODO(), runInput)
	if err != nil {
		return HandleFail(check, fmt.Sprintf("Could not create ec2 instance: %s", err))
	}

	if len(runOutput.Instances) != 1 {
		return HandleFail(check, fmt.Sprintf("Expected 1 instance, got: %v", len(runOutput.Instances)))
	}

	instanceID := runOutput.Instances[0].InstanceId
	ip := ""
	internalDNS := ""

	logger.Infof("Created EC2 instance with id %v", *instanceID)
	var startTime float64
	for {
		describeInput := &ec2.DescribeInstancesInput{InstanceIds: []string{*instanceID}}
		describeOutput, err := client.DescribeInstances(context.TODO(), describeInput)
		if err != nil {
			return HandleFail(check, fmt.Sprintf("Could not retrieve instance health: %s", err))
		}
		instance := describeOutput.Reservations[0].Instances[0]
		state := instance.State
		reason := instance.StateReason

		if state.Name == types.InstanceStateNameRunning {
			startTime = timeTracker.Elapsed()
			if describeOutput.Reservations[0].Instances[0].PublicIpAddress != nil {
				ip = *describeOutput.Reservations[0].Instances[0].PublicIpAddress
			}
			if describeOutput.Reservations[0].Instances[0].PrivateDnsName != nil {
				internalDNS = *describeOutput.Reservations[0].Instances[0].PrivateDnsName
			}
			break
		}
		if timeTracker.Millis() > 300000 {
			return HandleFail(check, fmt.Sprintf("Instance did not start within 5 minutes: %v", *reason.Message))
		}
		time.Sleep(1 * time.Second)
	}
	prometheusStartupTime.WithLabelValues(check.Region).Set(startTime)
	time.Sleep(time.Duration(check.WaitTime) * time.Second)
	fmt.Println(ip)

	var innerCanaries []v1.Canary

	innerFail := false
	var innerMessage []string

	for _, canary := range check.CanaryRef {
		innerCanary := v1.Canary{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Canary",
				APIVersion: "canaries.flanksource.com/v1",
			},
		}
		err = kommonsClient.Get(namespace, canary.Name, &innerCanary)
		logger.Infof("Accessing Canary %v/%v", namespace, canary.Name)
		if err != nil {
			innerFail = true
			innerMessage = append(innerMessage, fmt.Sprintf("Could not retrieve canary ref %v in %v: %v", canary.Name, namespace, err))
			break
		}
		if innerCanary.Name == "" {
			innerFail = true
			innerMessage = append(innerMessage, fmt.Sprintf("Could not retrieve canary ref %v in %v", canary.Name, namespace))
			break
		}
		innerCanaries = append(innerCanaries, innerCanary)
	}

	ec2Vars := map[string]string{
		"PublicIpAddress": ip,
		"instanceId":      *instanceID,
		"PrivateDnsName":  internalDNS,
	}

	for _, inner := range innerCanaries {
		inner.Spec = pkg.ApplyLocalTemplates(inner.Spec, ec2Vars)
		if inner.Spec.EC2.AMI != "" {
			logger.Warnf("EC2 checks may not be nested to avoid potential recursion.  Skipping inner EC2")
			inner.Spec.EC2.AMI = ""
		}
		innerResults := RunChecks(inner)
		for _, result := range innerResults {
			if !result.Pass {
				innerFail = true
				innerMessage = append(innerMessage, result.Message)
			}
		}
	}

	timeTracker = NewTimer()
	if !check.KeepAlive {
		err = terminateInstances(client, []string{*instanceID}, 300000)
		if err != nil {
			return HandleFail(check, fmt.Sprintf("Could not terminate: %s", err))
		}
	}
	stopTime := timeTracker.Elapsed()

	prometheusTerminateTime.WithLabelValues(check.Region).Set(stopTime)

	metricsList := []pkg.Metric{
		{
			Name:  "Startup Time",
			Value: startTime,
			Type:  metrics.GaugeType,
		},
		{
			Name:  "Termination Time",
			Value: stopTime,
			Type:  metrics.GaugeType,
		},
	}

	if innerFail {
		return HandleFail(check, fmt.Sprintf("referenced canaries failed: %v", strings.Join(innerMessage, ", ")))
	}

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Invalid:  false,
		Duration: int64(timeTracker.Elapsed()),
		Metrics:  metricsList,
	}
}

func HandleFail(check v1.EC2Check, message string) *pkg.CheckResult {
	prometheusFailCount.WithLabelValues(check.Region).Inc()
	return &pkg.CheckResult{ // nolint: staticcheck
		Check:       check,
		Pass:        false,
		Duration:    0,
		Invalid:     false,
		DisplayType: "Text",
		Message:     message,
	}
}

func terminateInstances(client *ec2.Client, instanceIds []string, timeout int64) error {
	timer := timer.NewTimer()
	terminateInput := &ec2.TerminateInstancesInput{InstanceIds: instanceIds}
	_, err := client.TerminateInstances(context.TODO(), terminateInput)

	if err != nil {
		return fmt.Errorf("terminate call error: %w", err)
	}

	for {
		describeInput := &ec2.DescribeInstancesInput{InstanceIds: instanceIds}
		describeOutput, err := client.DescribeInstances(context.TODO(), describeInput)
		if err != nil {
			return fmt.Errorf("describe call error: %w", err)
		}
		terminated := true
		var message []string
		for r := range describeOutput.Reservations {
			for i := range describeOutput.Reservations[r].Instances {
				state := *describeOutput.Reservations[r].Instances[i].State
				if state.Name != types.InstanceStateNameTerminated {
					terminated = false
					message = append(message, *describeOutput.Reservations[r].Instances[i].StateReason.Message)
				}
			}
		}
		if terminated {
			return nil
		}

		if timer.Millis() > timeout {
			return errors.New(strings.Join(message, "\n"))
		}
		time.Sleep(1 * time.Second)
	}
}

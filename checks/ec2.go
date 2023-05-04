//go:build !fast

package checks

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"encoding/base64"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/timer"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"net/http"

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

const defaultARN = "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2"

func init() {
	prometheus.MustRegister(prometheusCount, prometheusFailCount, prometheusPassCount, prometheusResponseTime, prometheusStartupTime, prometheusTerminateTime)
}

type EC2Checker struct {
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *EC2Checker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, ec2 := range ctx.Canary.Spec.EC2 {
		results = append(results, c.Check(ctx, ec2)...)
	}

	return results
}

// Type: returns checker type
func (c *EC2Checker) Type() string {
	return "ec2"
}

type AWS struct {
	EC2    *ec2.Client
	Config aws.Config
	ctx    *context.Context
}

func NewAWS(ctx *context.Context, check v1.EC2Check) (*AWS, error) {
	if err := check.AWSConnection.Populate(ctx, ctx.Kommons, ctx.Canary.GetNamespace()); err != nil {
		return nil, fmt.Errorf("failed to populate AWS connection: %v", err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: check.SkipTLSVerify},
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(check.AWSConnection.AccessKey.Value, check.AWSConnection.SecretKey.Value, "")),
		config.WithRegion(check.Region),
		config.WithHTTPClient(&http.Client{Transport: tr}),
	)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("failed to load AWS credentials: %v", err))
	}

	return &AWS{
		EC2:    ec2.NewFromConfig(cfg),
		Config: cfg,
		ctx:    ctx,
	}, nil
}

func (cfg *AWS) GetAMI(check v1.EC2Check) (*string, error) {
	if check.AMI != "" {
		return aws.String(check.AMI), nil
	}
	ssmClient := ssm.NewFromConfig(cfg.Config)
	arnLookupInput := &ssm.GetParameterInput{Name: aws.String(defaultARN)}
	arnLookupOutput, err := ssmClient.GetParameter(cfg.ctx, arnLookupInput)
	if err != nil {
		return nil, fmt.Errorf("could not look up amazon image arn: %v", err)
	}
	return arnLookupOutput.Parameter.Value, nil
}

func (cfg *AWS) GetExistingInstanceIds(idString string) ([]string, error) {
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

	describeOutput, err := cfg.EC2.DescribeInstances(cfg.ctx, describeInput)
	if err != nil {
		return nil, fmt.Errorf("could not perform prerun check: %v", err)
	}

	staleIds := []string{}
	for r := range describeOutput.Reservations {
		for i := range describeOutput.Reservations[r].Instances {
			staleIds = append(staleIds, *describeOutput.Reservations[r].Instances[i].InstanceId)
		}
	}
	return staleIds, nil
}

func (cfg *AWS) Launch(check v1.EC2Check, name, ami string) (string, *time.Duration, error) {
	start := NewTimer()
	userData := base64.StdEncoding.EncodeToString([]byte(check.UserData))

	if check.SecurityGroup == "" {
		check.SecurityGroup = "default"
	}

	runInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(ami),
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
						Value: aws.String(name),
					},
				},
			},
		},
		UserData: aws.String(userData),
		SecurityGroups: []string{
			check.SecurityGroup,
		},
	}

	runOutput, err := cfg.EC2.RunInstances(cfg.ctx, runInput)
	if err != nil {
		return "", nil, fmt.Errorf("could not create ec2 instance: %s", err)
	}

	if len(runOutput.Instances) != 1 {
		return "", nil, fmt.Errorf("expected 1 instance, got: %v", len(runOutput.Instances))
	}
	if check.TimeOut == 0 {
		check.TimeOut = 300
	}

	id := runOutput.Instances[0].InstanceId
	logger.Infof("Created EC2 instance with id %v", *id)
	return *id, start.Duration(), nil
}

func (cfg *AWS) TerminateInstances(instanceIds []string, timeout time.Duration) (*time.Duration, error) {
	start := NewTimer()
	if len(instanceIds) == 0 {
		return nil, nil
	}
	timer := timer.NewTimer()
	terminateInput := &ec2.TerminateInstancesInput{InstanceIds: instanceIds}
	_, err := cfg.EC2.TerminateInstances(cfg.ctx, terminateInput)

	if err != nil {
		return nil, fmt.Errorf("terminate call error: %w", err)
	}

	for {
		describeInput := &ec2.DescribeInstancesInput{InstanceIds: instanceIds}
		describeOutput, err := cfg.EC2.DescribeInstances(cfg.ctx, describeInput)
		if err != nil {
			return nil, fmt.Errorf("describe call error: %w", err)
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
			return start.Duration(), nil
		}

		if timer.Millis() > timeout.Milliseconds() {
			return nil, errors.New(strings.Join(message, "\n"))
		}
		time.Sleep(1 * time.Second)
	}
}

func (cfg *AWS) Describe(instanceID string, timeout time.Duration) (internalIP string, internalDNS string, err error) {
	timer := NewTimer()
	for {
		describeInput := &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}}
		describeOutput, err := cfg.EC2.DescribeInstances(cfg.ctx, describeInput)
		if err != nil {
			return "", "", fmt.Errorf("could not retrieve instance health: %s", err)
		}
		instance := describeOutput.Reservations[0].Instances[0]
		state := instance.State
		reason := instance.StateReason

		if state.Name == types.InstanceStateNameRunning {
			if describeOutput.Reservations[0].Instances[0].PublicIpAddress != nil {
				internalIP = *describeOutput.Reservations[0].Instances[0].PublicIpAddress
			}
			if describeOutput.Reservations[0].Instances[0].PrivateDnsName != nil {
				internalDNS = *describeOutput.Reservations[0].Instances[0].PrivateDnsName
			}
			break
		}
		if time.Since(timer.Start) > timeout {
			return "", "", fmt.Errorf("instance did not start within %v: %v", timeout, *reason.Message)
		}
		time.Sleep(1 * time.Second)
	}
	logger.Infof("Found IP for %s: %s (%s)", instanceID, internalIP, internalDNS)
	return
}

func (c *EC2Checker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.EC2Check)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	prometheusCount.WithLabelValues(check.Region).Inc()
	namespace := ctx.Canary.Namespace

	kommonsClient := ctx.Kommons
	if kommonsClient == nil {
		return results.Failf("commons client not configured for ec2 checker")
	}

	aws, err := NewAWS(ctx, check)
	if err != nil {
		return results.ErrorMessage(err)
	}

	ami, err := aws.GetAMI(check)
	if err != nil {
		return results.ErrorMessage(err)
	}

	idString := fmt.Sprintf("%v/%v", namespace, ctx.Canary.Name)

	ids, err := aws.GetExistingInstanceIds(idString)
	if err != nil {
		return results.ErrorMessage(err)
	}
	logger.Infof("Found %v stale ec2 instances (%s) - terminating", len(ids), strings.Join(ids, ","))
	if _, err := aws.TerminateInstances(ids, 5*time.Minute); err != nil {
		return results.ErrorMessage(err)
	}

	instanceID, launchTime, err := aws.Launch(check, idString, *ami)
	if err != nil {
		return results.ErrorMessage(err)
	}

	ip, dns, err := aws.Describe(instanceID, 5*time.Minute)
	if err != nil {
		return results.ErrorMessage(err)
	}
	prometheusStartupTime.WithLabelValues(check.Region).Set(launchTime.Seconds() * 1000)
	time.Sleep(time.Duration(check.WaitTime) * time.Second)

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

	ec2Vars := map[string]interface{}{
		"PublicIpAddress": ip,
		"instanceId":      instanceID,
		"PrivateDnsName":  dns,
	}

	for _, inner := range innerCanaries {
		if len(inner.Spec.EC2) > 0 {
			return results.Failf("EC2 checks may not be nested to avoid potential recursion.  Skipping inner EC2")
		}
		innerResults := RunChecks(ctx.New(ec2Vars))
		for _, result := range innerResults {
			if !result.Pass {
				innerFail = true
				innerMessage = append(innerMessage, result.Message)
			}
		}
	}

	var stopTime time.Duration
	if !check.KeepAlive {
		logger.Infof("Terminating instance id %s", instanceID)
		stopTime, err := aws.TerminateInstances([]string{instanceID}, 60*time.Second)
		if err != nil {
			return results.ErrorMessage(err)
		}
		prometheusTerminateTime.WithLabelValues(check.Region).Set(stopTime.Seconds() * 1000)
	}

	metricsList := []pkg.Metric{
		{
			Name:  "Startup Time",
			Value: launchTime.Seconds() * 1000,
			Type:  metrics.GaugeType,
		},
		{
			Name:  "Termination Time",
			Value: stopTime.Seconds() * 1000,
			Type:  metrics.GaugeType,
		},
	}

	if innerFail {
		return HandleFail(check, fmt.Sprintf("referenced canaries failed: %v", strings.Join(innerMessage, ", ")))
	}
	result.Metrics = metricsList

	return results
}

func HandleFail(check v1.EC2Check, message string) pkg.Results {
	prometheusFailCount.WithLabelValues(check.Region).Inc()
	return pkg.Results{ // nolint: staticcheck
		{
			Check:       check,
			Pass:        false,
			Duration:    0,
			Invalid:     false,
			DisplayType: "Text",
			Message:     message,
		},
	}
}

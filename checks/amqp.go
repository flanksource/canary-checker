package checks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gocontext "context"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"

	amqp "github.com/rabbitmq/amqp091-go"
)

var amqpLatency = prometheus.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "canary_check_amqp_latency_seconds",
		Help: "Duration of an entire AMQP check operation in seconds",
	},
	[]string{"amqp_url", "canary_name"},
)
var amqpSetupLatency = prometheus.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "canary_check_amqp_setup_latency_seconds",
		Help: "Duration of the consume portion of an AMQP operation",
	},
	[]string{"amqp_url", "canary_name", "amqp_role"},
)
var amqpTimedOutCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "canary_check_amqp_timed_out_total",
		Help: "Total number of AMQP requests that have timed out",
	},
	[]string{"amqp_url", "canary_name"},
)
var amqpBootstrapCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "canary_check_amqp_bootstrap_total",
		Help: "Total number of AMQP bootstrap operations",
	},
	[]string{"amqp_url", "canary_name"},
)
var amqpEnqueued = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "canary_check_amqp_enqueued_messages",
		Help: "Saturation of a given AMQP queue",
	},
	[]string{"amqp_url", "canary_name"},
)
var amqpConsumers = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "canary_check_amqp_consumers",
		Help: "Utilization of a given AMQP queue",
	},
	[]string{"amqp_url", "canary_name"},
)

// amqpTimeout is the maximum time allotted for an AMQP receive operation.
// Attempts exceeding this are reported as having failed. (Perhaps this should
// be configurable?)
const amqpTimeout = 10 * time.Second

func init() {
	prometheus.MustRegister(
		amqpLatency,
		amqpSetupLatency,
		amqpTimedOutCount,
		amqpBootstrapCount,
		amqpEnqueued,
		amqpConsumers,
	)
}

type AMQPChecker struct{}

func (c *AMQPChecker) Type() string {
	return "amqp"
}

func (c *AMQPChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.AMQP {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// amqpPrepURL normalizes an AMQP URI, possibly adding nonempty credentials.
// See also: https://www.rabbitmq.com/uri-query-parameters.html.
func amqpPrepURL(addr, user, pass string) (string, error) {
	parsed, err := amqp.ParseURI(addr)
	if err != nil {
		if !strings.Contains(err.Error(), " scheme ") {
			return "", err
		}
		if strings.Contains("://", addr) {
			return "", err
		}
		s := "amqp"
		if strings.Contains(addr, "5671") { // e.g. 15671
			s += "s"
		}
		parsed, err = amqp.ParseURI(s + "://" + addr)
		if err != nil {
			return "", err
		}
	}
	if user == "" {
		parsed.Username = "guest"
		parsed.Password = "guest"
	} else {
		parsed.Username = user
		parsed.Password = pass
	}
	return parsed.String(), nil
}

// amqpCheck is a convenience object for conducting an AMQP check.
type amqpCheck struct {
	v1.AMQPCheck
	addr    string
	sAddr   string // addr without authority.userinfo component
	body    string
	qStatus *amqp.Queue // stash these in case we add native metrics
	headers amqp.Table
	args    amqp.Table
}

// newAmqpCheck initializes an amqpCheck object.
func newAMQPCheck(check v1.AMQPCheck, ctx *context.Context) (*amqpCheck, error) {
	ac := &amqpCheck{AMQPCheck: check}
	// Initialize auth
	auth, err := GetAuthValues(ac.Auth, ctx.Kommons, ctx.Canary.Namespace)
	if err != nil {
		return nil, err
	}
	addr, err := amqpPrepURL(ac.Addr, auth.GetUsername(), auth.GetPassword())
	if err != nil {
		return nil, err
	}
	sAddr, err := amqpPrepURL(addr, "", "")
	if err != nil {
		return nil, err
	}
	ac.addr, ac.sAddr = addr, sAddr
	// Initialize body
	ac.body = ac.Simulation.Body
	if ac.body == "" {
		ts := time.Now().Format(time.RFC3339)
		var b string
		if ac.Exchange.Type == "" {
			b = `"auto"`
		} else {
			s, err := json.Marshal(ac.Binding)
			if err != nil {
				return nil, err
			}
			b = string(s)
		}
		ac.body = fmt.Sprintf(`{"binding":%s,"ts":%q}`, b, ts)
	}
	// Populate args
	if err := ac.desRawTables(); err != nil {
		return nil, err
	}
	return ac, nil
}

func (ac *amqpCheck) desRawTables() error {
	if ac.Binding.Args != nil {
		err := json.Unmarshal(ac.Binding.Args, &ac.args)
		if err != nil {
			return err
		}
	}
	if ac.Simulation.Headers != nil {
		err := json.Unmarshal(ac.Simulation.Headers, &ac.headers)
		if err != nil {
			return err
		}
	}
	return nil
}

// validate returns an error if the v1.AMQPCheck instance is problematic.
//
// TODO see if a native validation facility exists to handle this earlier.
// Ideally, it should only concern nonstandard fields introduced by us.
// However, the upstream library does not explain why it rejects various
// requests. For example: Exception (403) Reason: "ACCESS_REFUSED - operation
// not permitted on the default exchange".
func (ac *amqpCheck) validate() error {
	if ac.Exchange.Type == "" && ac.Exchange.Name != "" {
		return fmt.Errorf(
			"expected empty exchange.name with empty exchange.type %s, got %s",
			ac.Exchange.Type, ac.Exchange.Name,
		)
	}
	if ac.Exchange.Type != "" && ac.Exchange.Name == "" {
		return fmt.Errorf(
			"expected nonempty exchange.name with exchange.type %s",
			ac.Exchange.Type,
		)
	}
	if ac.Exchange.Type == "fanout" && ac.Simulation.Key != "" {
		return fmt.Errorf(
			"expected empty simulation.key with exchange.type fanout, got %s",
			ac.Simulation.Key,
		)
	}
	if ac.isAnonPubSub() && ac.Queue.Name != "" {
		return fmt.Errorf(
			"expected empty queue.name with empty exchange.type, got %s",
			ac.Queue.Name,
		)
	}
	if ac.Simulation.Bootstrap && ac.Simulation.Witness {
		return fmt.Errorf(
			"expected simulation fields witness and bootstrap to be exclusive",
		)
	}
	return nil
}

// open creates a new connection and an exchange.
// Supported types are "direct," "fanout," "topic," and "", for the default
// exchange (automatic binding, etc.).
func (ac *amqpCheck) open() (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(ac.addr)
	if err != nil {
		return nil, nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, err
	}
	return conn, ch, nil
}

// ensureExchange ensures an exchange with the appropriate params exists.
func (ac *amqpCheck) ensureExchange(ch *amqp.Channel) error {
	if ac.Exchange.Type != "" {
		err := ch.ExchangeDeclare(
			ac.Exchange.Name,
			ac.Exchange.Type,
			ac.Exchange.Durable,
			ac.Exchange.AutoDelete,
			false,
			false,
			nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureQueue declares a queue, and optionally binds it.
func (ac *amqpCheck) ensureQueue(ch *amqp.Channel) (*amqp.Queue, error) {
	q, err := ch.QueueDeclare(
		ac.Queue.Name,
		ac.Queue.Durable,
		ac.Queue.AutoDelete,
		ac.isAnonPubSub(),
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}
	qStatus, err := ch.QueueInspect(q.Name)
	if err != nil {
		return nil, err
	}
	ac.qStatus = &qStatus
	ac.qStatus.Consumers++ // Include ourselves
	amqpEnqueued.WithLabelValues(ac.sAddr, ac.Name).Set(float64(qStatus.Messages))
	amqpConsumers.WithLabelValues(ac.sAddr, ac.Name).Set(float64(qStatus.Consumers))
	if ac.isAnonPubSub() || ac.Peek || ac.Binding.Key != "" || ac.args != nil {
		name := ac.Binding.Name
		if name == "" {
			name = q.Name
		}
		err := ch.QueueBind(name, ac.Binding.Key, ac.Exchange.Name, false, ac.args)
		if err != nil {
			return nil, err
		}
	}
	return &q, nil
}

// isAnonPubSub indicates whether we're simulating a pub-sub sequence.
// This implies automatically named "exclusive" queues will be used.
func (ac *amqpCheck) isAnonPubSub() bool {
	return !ac.Peek && ac.Exchange.Type != ""
}

// push publishes to a queue.
func (ac *amqpCheck) push(ctx *context.Context) error {
	timer := prometheus.NewTimer(
		amqpSetupLatency.WithLabelValues(ac.sAddr, ac.Name, "producer"),
	)
	conn, ch, err := ac.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	if err := ac.ensureExchange(ch); err != nil {
		return err
	}
	k := ac.Simulation.Key
	if !ac.Simulation.Witness {
		queue, err := ac.ensureQueue(ch)
		if err != nil {
			return err
		}
		if k == "" {
			k = queue.Name
		}
	}
	timer.ObserveDuration()
	if ac.Simulation.Bootstrap {
		if ac.qStatus.Messages != 0 {
			return nil
		}
		amqpBootstrapCount.WithLabelValues(ac.sAddr, ac.Name).Inc()
	}
	// (Unverified) using ctx.WithTimeout instead here seems to cause issues
	// involving duplicate checks.
	goctx, cancel := gocontext.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()
	payload := amqp.Publishing{
		Headers:     ac.headers, // service will type check
		ContentType: "text/plain",
		Body:        []byte(ac.body),
	}
	return ch.PublishWithContext(
		goctx,
		ac.Exchange.Name,
		k,
		false,
		false,
		payload,
	)
}

// receive waits for a message to arrive and validates it.
func (ac *amqpCheck) receive(ds <-chan amqp.Delivery) error {
	select {
	case d := <-ds:
		if ac.Peek {
			if err := d.Reject(true); err != nil {
				return err
			}
			return nil
		}
		if ac.Ack {
			if err := d.Ack(false); err != nil {
				return err
			}
		}
		if ac.body != "" && string(d.Body) != ac.body {
			return fmt.Errorf("got unexpected message: %s", string(d.Body))
		}
		return nil
	case <-time.After(amqpTimeout):
		amqpTimedOutCount.WithLabelValues(ac.sAddr, ac.Name).Inc()
		return fmt.Errorf("timed out waiting message")
	}
}

// pull consumes messages.
// This also verifies whether creation properties match. Keys aren't
// considered, however, so additional bindings may be created.
func (ac *amqpCheck) pull(r func(<-chan amqp.Delivery) error) error {
	timer := prometheus.NewTimer(
		amqpSetupLatency.WithLabelValues(ac.sAddr, ac.Name, "consumer"),
	)
	conn, ch, err := ac.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	if err := ac.ensureExchange(ch); err != nil {
		return err
	}
	queue, err := ac.ensureQueue(ch)
	if err != nil {
		return err
	}
	timer.ObserveDuration()
	autoAck := ac.isAnonPubSub() || (!ac.Ack && !ac.Peek)
	ds, err := ch.Consume(queue.Name, "", autoAck, false, false, false, nil)
	if err != nil {
		return err
	}
	err = r(ds)
	if err != nil {
		return err
	}
	return nil
}

// witness performs a publish-subscribe sequence.
// It differs from pushPull in that the subscriber connects first to intercept
// a published message. Currently, this is only used for non-default, non-peek
// runs. Despite the name, the exchange type can be something other than
// topic.
func (ac *amqpCheck) witness(ctx *context.Context) error {
	return ac.pull(
		func(ds <-chan amqp.Delivery) error {
			if err := ac.push(ctx); err != nil {
				return err
			}
			return ac.receive(ds)
		},
	)
}

// pushPull performs a basic dialog but only consumes when "peeking."
// This differs from witness in that the producer first runs to completion
// (unless "peeking").
func (ac *amqpCheck) pushPull(ctx *context.Context) error {
	if !ac.Peek || ac.Simulation.Bootstrap {
		if err := ac.push(ctx); err != nil {
			return err
		}
	}
	if err := ac.pull(ac.receive); err != nil {
		return err
	}
	return nil
}

func (c *AMQPChecker) Check(ctx *context.Context, ec external.Check) pkg.Results {
	check := ec.(v1.AMQPCheck)
	results := pkg.Results{pkg.Success(check, ctx.Canary)}
	ac, err := newAMQPCheck(check, ctx)
	if err != nil {
		return results.Failf("failed to initialize amqpCheck object: %v", err)
	}
	if err := ac.validate(); err != nil {
		return results.Failf("failed to validate AMQPCheck: %v", err)
	}
	timer := prometheus.NewTimer(amqpLatency.WithLabelValues(ac.sAddr, ac.Name))
	if ac.Simulation.Witness {
		err = ac.witness(ctx)
	} else {
		err = ac.pushPull(ctx)
	}
	if err != nil {
		results = results.Failf("failed to perform AMQP dialog: %v", err)
	}
	timer.ObserveDuration()
	// Add queue stats
	if ac.qStatus != nil {
		results[0].AddMetric(pkg.Metric{
			Name:   "enqueued",
			Type:   metrics.GaugeType,
			Labels: map[string]string{"endpoint": ac.sAddr},
			Value:  float64(ac.qStatus.Messages),
		}).AddMetric(pkg.Metric{
			Name:   "consumers",
			Type:   metrics.GaugeType,
			Labels: map[string]string{"endpoint": ac.sAddr},
			Value:  float64(ac.qStatus.Consumers),
		})
	}
	return results
}

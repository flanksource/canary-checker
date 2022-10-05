package checks

import (
	"fmt"
	"strings"
	"time"

	gocontext "context"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/prometheus/client_golang/prometheus"

	amqp "github.com/rabbitmq/amqp091-go"
)

var amqpLatency = prometheus.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "canary_check_amqp_latency_seconds",
		Help: "Duration of an AMQP operation in seconds",
	},
	[]string{"amqp_url", "canary_name"},
)

// amqpTimeout is the maximum time allotted for an AMQP receive operation.
// Attempts exceeding this are reported as having failed. (Perhaps this should
// be configurable?)
const amqpTimeout = 10 * time.Second

func init() {
	prometheus.MustRegister(amqpLatency)
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
	addr  string
	sAddr string // addr without authority.userinfo component
	body  string
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
	ts := time.Now().Format(time.RFC3339)
	key := "auto"
	if ac.Exchange.Type != "" {
		key = ac.getKey(false)
	}
	ac.body = fmt.Sprintf(`{"bind":"%s","ts":"%s"}`, key, ts)
	return ac, nil
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
			ac.getExchangeName(),
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
	queue, err := ch.QueueDeclare(
		ac.getQueueName(),
		ac.Queue.Durable,
		ac.Queue.AutoDelete,
		ac.isAnonPubSub(),
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}
	if ac.isAnonPubSub() || ac.Peek {
		x := ac.getExchangeName()
		k := ac.getKey(false) // always false, even for topics
		if err := ch.QueueBind(queue.Name, k, x, false, nil); err != nil {
			return nil, err
		}
	}
	return &queue, nil
}

// getKey returns a plausible key given the configured exchange type.
// Param isProducer should be false if irrelevant or the caller is uncertain.
func (ac *amqpCheck) getKey(isProducer bool) string {
	if ac.Key != "" {
		return ac.Key
	}
	switch ac.Exchange.Type {
	case "direct":
		return "foo"
	case "topic":
		if isProducer {
			return "canary.check"
		}
		return "canary.*"
	default: // fanout
		return ""
	}
}

// isAnonPubSub says whether an ephemeral, non-default exchange was specified.
// This implies automatically named (exclusive) queues will be used.
func (ac *amqpCheck) isAnonPubSub() bool {
	return !ac.Peek && ac.Exchange.Type != ""
}

// getExchangeName returns the name of the exchange.
func (ac *amqpCheck) getExchangeName() string {
	if ac.Exchange.Name != "" {
		return ac.Exchange.Name
	}
	if ac.Exchange.Type == "" {
		return ""
	}
	return "canary." + ac.Exchange.Type
}

// getQueueName returns the name of the active queue.
func (ac *amqpCheck) getQueueName() string {
	if ac.isAnonPubSub() {
		return ""
	}
	if ac.Queue.Name != "" {
		return ac.Queue.Name
	}
	return "check"
}

// push publishes to a queue.
func (ac *amqpCheck) push(ctx *context.Context) error {
	conn, ch, err := ac.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	if err := ac.ensureExchange(ch); err != nil {
		return err
	}
	k := ac.getKey(true)
	if !ac.isAnonPubSub() {
		queue, err := ac.ensureQueue(ch)
		if err != nil {
			return err
		}
		if ac.Queue.Name == "" {
			k = queue.Name
		}
	}
	// XXX as of 15e7cea06125f5df9ad0c07e2e73f1b8f498ce39 v0.38.171, using
	// ctx.WithTimeout here appears to create duplicate checks that interfere
	// with one another. If this isn't intentional, perhaps it has to do with
	// api/context/context.go's Context.WithDeadline replacing its embedded
	// stdlib Context with a child?
	goctx, cancel := gocontext.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()
	payload := amqp.Publishing{ContentType: "text/plain", Body: []byte(ac.body)}
	err = ch.PublishWithContext(
		goctx,
		ac.getExchangeName(),
		k,
		false,
		false,
		payload,
	)
	if err != nil {
		return err
	}
	return nil
}

// receive waits for a message to arrive and validates it.
func (ac *amqpCheck) receive(ds <-chan amqp.Delivery) error {
	select {
	case d := <-ds:
		if !ac.isAnonPubSub() && ac.Peek {
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
		if string(d.Body) != ac.body {
			return fmt.Errorf("got unexpected message: %s", string(d.Body))
		}
		return nil
	case <-time.After(amqpTimeout):
		return fmt.Errorf("timed out waiting message")
	}
}

// pull consumes messages.
// This also verifies whether creation properties match. Keys aren't
// considered, however, so additional bindings may be created.
func (ac *amqpCheck) pull(r func(<-chan amqp.Delivery) error) error {
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
	autoAck := ac.isAnonPubSub() || (!ac.Ack && !ac.Peek)
	ds, err := ch.Consume(queue.Name, "", autoAck, false, false, false, nil)
	if err != nil {
		return err
	}
	if err := r(ds); err != nil {
		return err
	}
	return nil
}

// pubSub performs a publish-subscribe sequence.
// It differs from pushPull in that the subscriber connects first to intercept
// a published message. Currently, this is only used for non-default, non-peek
// runs. Despite the name, the exchange type can be something other than
// topic.
func (ac *amqpCheck) pubSub(ctx *context.Context) error {
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
// This differs from pubSub in that the producer first runs to completion
// (unless "peeking").
func (ac *amqpCheck) pushPull(ctx *context.Context) error {
	if !ac.Peek {
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
	timer := prometheus.NewTimer(amqpLatency.WithLabelValues(ac.sAddr, ac.Name))
	if ac.isAnonPubSub() {
		err = ac.pubSub(ctx)
	} else {
		err = ac.pushPull(ctx)
	}
	if err != nil {
		results = results.Failf("failed to perform AMQP dialog: %v", err)
	}
	timer.ObserveDuration()
	return results
}

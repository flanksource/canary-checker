package checks

// FIXME: delete this file after covering everything in e2e.

import (
	gocontext "context"
	"fmt"
	"os"
	"testing"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
)

var username, password string
var host string

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
	if p := os.Getenv("RABBITMQ_PASSWORD"); p != "" {
		password = p
	}
	if u := os.Getenv("RABBITMQ_USERNAME"); u != "" {
		username = u
	}
	if h := os.Getenv("RABBITMQ_HOSTNAME"); h != "" {
		host = h
	} else {
		host = "localhost"
	}
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

func TestAmqpPushPull(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	ctx := &context.Context{Context: gocontext.TODO()}
	addr, err := amqpPrepURL(host, username, password)
	if err != nil {
		t.Fatal(err)
	}
	// Default
	qu := v1.AMQPQueue{Name: "check"}
	ac := &amqpCheck{v1.AMQPCheck{Queue: qu}, addr, "", t.Name(), nil}
	if err := ac.validate(); err != nil {
		t.Fatal(err)
	}
	err = ac.pushPull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// ACK'd
	ac = &amqpCheck{v1.AMQPCheck{Queue: qu, Ack: true}, addr, "", t.Name(), nil}
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
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	ctx := &context.Context{Context: gocontext.TODO()}
	addr, err := amqpPrepURL(host, username, password)
	if err != nil {
		t.Fatal(err)
	}
	// Blind multicast
	aC := v1.AMQPCheck{
		Exchange:   v1.AMQPExchange{Name: "canary.fanout", Type: "fanout"},
		Simulation: v1.AMQPSimulation{Witness: true},
	}
	ac := &amqpCheck{aC, addr, "", t.Name(), nil}
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
	ac.Binding = ac.Simulation.Key
	ac = &amqpCheck{aC, addr, "", t.Name(), nil}
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
	ac.Binding = "topic.*"
	ac = &amqpCheck{aC, addr, "", t.Name(), nil}
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

func runAMQPPeek(kind, xName, qName, bindKey, sendKey string, bs bool) error {
	ctx := &context.Context{Context: gocontext.TODO()}
	addr, err := amqpPrepURL(host, username, password)
	if err != nil {
		return err
	}
	aC := v1.AMQPCheck{
		Exchange:   v1.AMQPExchange{Name: xName, Type: kind},
		Queue:      v1.AMQPQueue{Name: qName},
		Simulation: v1.AMQPSimulation{Key: sendKey},
		Binding:    bindKey,
		Peek:       true,
	}
	ac := &amqpCheck{aC, addr, "", "body=" + qName, nil}
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
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	if err := runAMQPPeek(
		"direct", t.Name()+".normal", t.Name(), "foo", "foo", false,
	); err != nil {
		t.Fatal(err)
	}
	if err := runAMQPPeek(
		"direct", t.Name()+".bootstrap", t.Name(), "foo", "foo", true,
	); err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekTopic(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	if err := runAMQPPeek(
		"topic", t.Name()+".normal", t.Name(), "foo.*", "foo.bar", false,
	); err != nil {
		t.Fatal(err)
	}
	if err := runAMQPPeek(
		"topic", t.Name()+".bootstrap", t.Name(), "foo.*", "foo.bar", true,
	); err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekFanout(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	err := runAMQPPeek("fanout", t.Name()+".normal", t.Name(), "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	err = runAMQPPeek("fanout", t.Name()+".bootstrap", t.Name(), "", "", true)
	if err != nil {
		t.Fatal(err)
	}
}

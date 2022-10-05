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
	ac := &amqpCheck{v1.AMQPCheck{}, addr, "", t.Name()}
	err = ac.pushPull(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// ACK'd
	ac = &amqpCheck{v1.AMQPCheck{Ack: true}, addr, "", t.Name()}
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

func TestAmqpPubSub(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	ctx := &context.Context{Context: gocontext.TODO()}
	addr, err := amqpPrepURL(host, username, password)
	if err != nil {
		t.Fatal(err)
	}
	// Blind multicast
	ex := v1.AMQPExchange{Type: "fanout"}
	ac := &amqpCheck{v1.AMQPCheck{Exchange: ex}, addr, "", t.Name()}
	err = ac.pubSub(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Explicit targeting/routing
	ex = v1.AMQPExchange{Type: "direct"}
	ac = &amqpCheck{v1.AMQPCheck{Exchange: ex}, addr, "", t.Name()}
	err = ac.pubSub(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Pattern matching
	ex = v1.AMQPExchange{Type: "topic"}
	ac = &amqpCheck{v1.AMQPCheck{Exchange: ex}, addr, "", t.Name()}
	err = ac.pubSub(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Delete
	err = deleteExchanges(ac, "canary.direct", "canary.fanout", "canary.topic")
	if err != nil {
		t.Fatal(err)
	}
}

func runAMQPPeek(kind, exchangeName, queueName, bindKey, sendKey string) error {
	ctx := &context.Context{Context: gocontext.TODO()}
	addr, err := amqpPrepURL(host, username, password)
	if err != nil {
		return err
	}
	ex := v1.AMQPExchange{Name: exchangeName, Type: kind}
	qu := v1.AMQPQueue{Name: queueName}
	aC := v1.AMQPCheck{Exchange: ex, Queue: qu, Key: bindKey, Peek: true}
	ac := &amqpCheck{aC, addr, "", queueName}
	// Prepare
	err = ac.push(ctx)
	if err != nil {
		return err
	}
	aC.Key = sendKey
	// Default
	err = ac.pushPull(ctx)
	if err != nil {
		return err
	}
	// Again
	err = ac.pushPull(ctx)
	if err != nil {
		return err
	}
	// Delete
	n, err := deleteQueue(ac, queueName)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("Wrong number of messages remaining: %d", n)
	}
	err = deleteExchanges(ac, exchangeName)
	if err != nil {
		return err
	}
	return nil
}

func TestAmqpPeekDirect(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	err := runAMQPPeek("direct", t.Name()+".direct", t.Name(), "foo", "foo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekTopic(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	err := runAMQPPeek("topic", t.Name()+".topic", t.Name(), "foo.*", "foo.bar")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAmqpPeekFanout(t *testing.T) {
	if username == "" || password == "" {
		t.Skip("Missing env var(s) RABBITMQ_{USERNAME,PASSWORD}")
	}
	err := runAMQPPeek("fanout", t.Name()+".fanout", t.Name(), "", "")
	if err != nil {
		t.Fatal(err)
	}
}

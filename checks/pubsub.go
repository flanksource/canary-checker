package checks

import (
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"gocloud.dev/pubsub"
)

type PubSubChecker struct {
}

func (c *PubSubChecker) Type() string {
	return "pubsub"
}

func (c *PubSubChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.PubSub {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *PubSubChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.PubSubCheck)
	switch check.Type {
	case "gcp_incidents":
		return CheckGCPIncidents(ctx, check)
	}
	return pkg.Results{}
}

type PubSubResults struct {
	GCPIncidents []GCPIncident `json:"gcp_incidents"`
}

func ListenWithTimeout(ctx *context.Context, subscription *pubsub.Subscription, timeout time.Duration) ([]string, error) {
	timeoutCh := make(chan bool, 1)
	messageCh := make(chan string, 1)
	errorCh := make(chan error, 1)

	var messages []string

	for {
		// Reset after each iteration
		timer := time.AfterFunc(timeout, func() {
			timeoutCh <- true
		})

		// Listen for messages in a goroutine
		go func() {
			msg, err := subscription.Receive(ctx)
			if err != nil {
				errorCh <- err
				return
			}
			messageCh <- string(msg.Body)
			msg.Ack()
		}()

		// Wait for either a message, error, or timeout
		select {
		case <-ctx.Done():
			return messages, nil
		case msg := <-messageCh:
			// Stop the timer since we got a message
			timer.Stop()
			messages = append(messages, msg)
		case err := <-errorCh:
			timer.Stop()
			return messages, err
		case <-timeoutCh:
			return messages, nil
		}
	}
}

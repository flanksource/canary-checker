package checks

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/pubsub"
	gocloudpubsub "gocloud.dev/pubsub"
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
	var results pkg.Results
	check := extConfig.(v1.PubSubCheck)
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	subscription, err := pubsub.Subscribe(ctx.Context, check.QueueConfig)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("error opening subscription for %s: %w", check.GetQueue()))
	}

	defer subscription.Shutdown(ctx) //nolint:errcheck

	msgs, err := ListenWithTimeout(ctx, subscription, 10*time.Second)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("error listening to subscription %s: %w", check.GetQueue(), err))
	}

	result.AddDetails(PubSubResults{Messages: msgs})
	return results
}

type PubSubResults struct {
	Messages []string `json:"messages"`
}

func ListenWithTimeout(ctx *context.Context, subscription *gocloudpubsub.Subscription, timeout time.Duration) ([]string, error) {
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

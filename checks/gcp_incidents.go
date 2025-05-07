package checks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/gcppubsub"
)

func Check() error {

	// Pull from topic, and create messages
	// {
	_ = `
{
  "incident": {
    "condition": {
      "conditionThreshold": {
        "aggregations": [
          {
            "alignmentPeriod": "120s",
            "perSeriesAligner": "ALIGN_MIN"
          }
        ],
        "comparison": "COMPARISON_GT",
        "duration": "0s",
        "filter": "resource.type = \"gce_instance\" AND metric.type = \"compute.googleapis.com/instance/network/sent_packets_count\"",
        "thresholdValue": 2001,
        "trigger": {
          "count": 1
        }
      },
      "displayName": "VM Instance - Sent packets",
      "name": "projects/flanksource-sandbox/alertPolicies/12942885046098191375/conditions/7238353791745156150"
    },
    "condition_name": "VM Instance - Sent packets",
    "documentation": {
      "content": "",
      "mime_type": "",
      "subject": "[ALERT - Error] Incident created"
    },
    "ended_at": null,
    "incident_id": "0.nrwotmyokd9n",
    "metadata": {
      "system_labels": {},
      "user_labels": {}
    },
    "metric": {
      "displayName": "Sent packets",
      "labels": {
        "instance_name": "gke-hub-cluster-private-pool-containe-de26c8a2-0se1",
        "loadbalanced": "false"
      },
      "type": "compute.googleapis.com/instance/network/sent_packets_count"
    },
    "observed_value": "7679.000",
    "policy_name": "Test-incident-canary",
    "resource": {
      "labels": {
        "instance_id": "1943820386402233898",
        "project_id": "flanksource-sandbox",
        "zone": "europe-west1-b"
      },
      "type": "gce_instance"
    },
    "resource_display_name": "gke-hub-cluster-private-pool-containe-de26c8a2-0se1",
    "resource_id": "",
    "resource_name": "flanksource-sandbox gke-hub-cluster-private-pool-containe-de26c8a2-0se1",
    "resource_type_display_name": "VM Instance",
    "scoping_project_id": "flanksource-sandbox",
    "scoping_project_number": 365415247865,
    "severity": "Error",
    "started_at": 1746178794,
    "state": "open",
    "summary": "Sent packets for flanksource-sandbox gke-hub-cluster-private-pool-containe-de26c8a2-0se1 with metric labels {instance_name=gke-hub-cluster-private-pool-containe-de26c8a2-0se1, loadbalanced=false} is above the threshold of 2001.000 with a value of 7679.000.",
    "threshold_value": "2001",
    "url": "https://console.cloud.google.com/monitoring/alerting/alerts/0.nrwotmyokd9n?channelType=cloud-pubsub&project=flanksource-sandbox"
  },
  "version": "1.2"
}
	`

	_ = `  "incident": {
    "condition": {
      "conditionThreshold": {
        "aggregations": [
          {
            "alignmentPeriod": "300s",
            "perSeriesAligner": "ALIGN_MIN"
          }
        ],
        "comparison": "COMPARISON_GT",
        "duration": "0s",
        "filter": "resource.type = \"gce_instance\" AND metric.type = \"compute.googleapis.com/instance/network/sent_packets_count\"",
        "thresholdValue": 2000,
        "trigger": {
          "count": 1
        }
      },
      "displayName": "VM Instance - Sent packets",
      "name": "projects/flanksource-sandbox/alertPolicies/12942885046098191375/conditions/12942885046098193590"
    },
    "condition_name": "VM Instance - Sent packets",
    "documentation": {
      "content": "",
      "mime_type": "",
      "subject": "[RESOLVED - Error] Incident created"
    },
    "ended_at": 1746178747,
    "incident_id": "0.nrwo9fe1a4wg",
    "metadata": {
      "system_labels": {},
      "user_labels": {}
    },
    "metric": {
      "displayName": "Sent packets",
      "labels": {
        "instance_name": "gke-hub-cluster-private-pool-containe-00531631-rf93",
        "loadbalanced": "false"
      },
      "type": "compute.googleapis.com/instance/network/sent_packets_count"
    },
    "observed_value": "Incident cancelled because the alert policy condition was deleted or modified while incident was active.",
    "policy_name": "Test-incident-canary",
    "resource": {
      "labels": {
        "instance_id": "8733652352636743443",
        "project_id": "flanksource-sandbox",
        "zone": "europe-west1-c"
      },
      "type": "gce_instance"
    },
    "resource_display_name": "gke-hub-cluster-private-pool-containe-00531631-rf93",
    "resource_id": "",
    "resource_name": "flanksource-sandbox gke-hub-cluster-private-pool-containe-00531631-rf93",
    "resource_type_display_name": "VM Instance",
    "scoping_project_id": "flanksource-sandbox",
    "scoping_project_number": 365415247865,
    "severity": "Error",
    "started_at": 1746177319,
    "state": "closed",
    "summary": "Sent packets for flanksource-sandbox gke-hub-cluster-private-pool-containe-00531631-rf93 with metric labels {instance_name=gke-hub-cluster-private-pool-containe-00531631-rf93, loadbalanced=false} returned to normal with a value of Incident cancelled because the alert policy condition was deleted or modified while incident was active..",
    "threshold_value": "2000",
    "url": "https://console.cloud.google.com/monitoring/alerting/alerts/0.nrwo9fe1a4wg?channelType=cloud-pubsub&project=flanksource-sandbox"
  },
  "version": "1.2"
}
`

	//for {
	//m, err := subscription.Receive(ctx)
	//}
	return nil
}

type GCPIncidentsChecker struct {
}

func (c *GCPIncidentsChecker) Type() string {
	return "gcp_incidents"
}

func (c *GCPIncidentsChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.GCPIncidents {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

type GCPIncident struct {
	ID      string `json:"incident_id,omitempty"`
	Summary string `json:"summary,omitempty"`
	State   string `json:"state,omitempty"`
}

func (g GCPIncident) ToMapStringAny() map[string]any {
	b, _ := json.Marshal(g)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

type GCPIncidentPayload struct {
	Incident GCPIncident `json:"incident,omitempty"`
}

func (c *GCPIncidentsChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.GCPIncidents)
	var results pkg.Results
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	subscription, err := pubsub.OpenSubscription(ctx, fmt.Sprintf("gcppubsub://projects/%s/subscriptions/%s", check.Project, check.Subscription))
	if err != nil {
		return results.WithError(fmt.Errorf("error opening subscription for %s/%s: %w", check.Project, check.Subscription, err))
	}
	defer subscription.Shutdown(ctx)

	msgs, err := ListenWithTimeout(ctx, subscription, 10*time.Second)
	if err != nil {
		panic(err)
	}

	logger.Infof("%+v", msgs)

	var g GCPIncidentList

	var results2 pkg.Results
	for _, rawMsg := range msgs {
		var k GCPIncident
		_ = json.Unmarshal([]byte(rawMsg), &k)

		// We ignore acknowledgement state
		if k.State == "closed" {
			g.closed = append(g.closed, k)
			if err := ctx.DB().Table("check_statuses").Where("detail->'id' = ?", k.ID).UpdateColumn("status", true).Error; err != nil {
				panic(err)
			}
		} else if k.State == "open" {
			g.Incidents = append(g.Incidents, k)
			rr := pkg.New(check, ctx.Canary)
			rr.Name = fmt.Sprintf("[%s] - %s", k.ID, k.Summary)
			rr.Pass = false
			rr.Data = k.ToMapStringAny()
			rr.Message = k.Summary
			rr.Detail = k.ToMapStringAny()
			results2 = append(results2, rr)
		}
	}

	return results2
}

type GCPIncidentList struct {
	Incidents []GCPIncident `json:"incidents,omitempty"`
	closed    []GCPIncident
}

func ListenWithTimeout(ctx *context.Context, subscription *pubsub.Subscription, timeout time.Duration) ([]string, error) {
	// Create a channel for timeout
	timeoutCh := make(chan bool, 1)
	messageCh := make(chan string, 1)
	errorCh := make(chan error, 1)

	var messages []string

	for {
		// Reset timer for each iteration
		timer := time.AfterFunc(timeout, func() {
			logger.Infof("Timeout")
			timeoutCh <- true
		})

		// Start a goroutine to listen for messages
		go func() {
			logger.Infof("Waiting to receive")
			msg, err := subscription.Receive(ctx)
			if err != nil {
				errorCh <- err
				return
			}
			logger.Infof("Got something %s", string(msg.Body))
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
			logger.Infof("Timeout done")
			return messages, nil
		}
	}
}

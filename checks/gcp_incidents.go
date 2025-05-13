package checks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"gocloud.dev/gcp"
	"gocloud.dev/pubsub/gcppubsub"
	"golang.org/x/oauth2"
)

/*
Sample open incident body
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

Sample closed incident body
{
  "incident": {
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

*/

type GCPIncident struct {
	ID           string `json:"incident_id,omitempty"`
	Summary      string `json:"summary,omitempty"`
	State        string `json:"state,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`

	m map[string]any
}

func (g *GCPIncident) ToMapStringAny() map[string]any {
	if g.m == nil {
		b, _ := json.Marshal(g)
		_ = json.Unmarshal(b, &g.m)
	}
	return g.m
}

type GCPIncidentList struct {
	Incidents []GCPIncident `json:"incidents,omitempty"`
}

type GCPIncidentPayload struct {
	Incident GCPIncident `json:"incident,omitempty"`
}

func CheckGCPIncidents(ctx *context.Context, extConfig external.Check) pkg.Results {
	var results pkg.Results
	pubSubCheck := extConfig.(v1.PubSubCheck)
	check := pubSubCheck.GCPIncidents
	result := pkg.Success(pubSubCheck, ctx.Canary)
	results = append(results, result)

	var tokenSrc oauth2.TokenSource
	if check.ConnectionName != "" {
		err := check.GCPConnection.HydrateConnection(ctx)
		if err != nil {
			return results.ErrorMessage(fmt.Errorf("error hydrating connection %s: %w", check.ConnectionName, err))
		}
		tokenSrc, err = check.GCPConnection.TokenSource(ctx.Context)
		if err != nil {
			return results.ErrorMessage(fmt.Errorf("error getting token source for %s/%s: %w", check.Project, check.Subscription, err))
		}
	} else {
		creds, err := gcp.DefaultCredentials(ctx)
		if err != nil {
			return results.ErrorMessage(fmt.Errorf("error creating default creds for %s/%s: %w", check.Project, check.Subscription, err))
		}
		tokenSrc = creds.TokenSource
	}

	conn, cleanup, err := gcppubsub.Dial(ctx, tokenSrc)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("error connecting to GCP: %w", err))
	}
	defer cleanup()

	subClient, err := gcppubsub.SubscriberClient(ctx, conn)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("error opening subscription for %s/%s: %w", check.Project, check.Subscription, err))
	}
	defer subClient.Close()

	subscription, err := gcppubsub.OpenSubscriptionByPath(subClient, fmt.Sprintf("projects/%s/subscriptions/%s", check.Project, check.Subscription), nil)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("error opening subscription for %s/%s: %w", check.Project, check.Subscription, err))
	}
	//nolint:errcheck
	defer subscription.Shutdown(ctx)

	msgs, err := ListenWithTimeout(ctx, subscription, 10*time.Second)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("error listening to subscription for %s/%s: %w", check.Project, check.Subscription, err))
	}

	var pubSubResult PubSubResults
	for _, rawMsg := range msgs {
		var payload GCPIncidentPayload
		_ = json.Unmarshal([]byte(rawMsg), &payload)

		inc := payload.Incident
		// We ignore acknowledgement state
		switch inc.State {
		case "closed":
			// We set the status to true directly in database
			if ctx.DB() != nil {
				if pubSubCheck.ResultLookup == "" {
					pubSubCheck.ResultLookup = `$.incident_id`
				}
				whereClause := fmt.Sprintf(`trim('"' FROM jsonb_path_query_first(details, '%s')::text) = ?`, pubSubCheck.ResultLookup)
				if err := ctx.DB().Table("check_statuses").Where(whereClause, inc.ID).UpdateColumn("status", true).Error; err != nil {
					return results.ErrorMessage(fmt.Errorf("error updating to subscription for %s/%s: %w", check.Project, check.Subscription, err))
				}
			}
		case "open":
			pubSubResult.GCPIncidents = append(pubSubResult.GCPIncidents, inc)
		}
	}

	result.AddDetails(pubSubResult)
	return results
}

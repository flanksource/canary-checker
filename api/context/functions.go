package context

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/uuid"
)

func (ctx *Context) GetContextualFunctions() map[string]any {
	funcs := make(map[string]any)
	if check, ok := ctx.Environment["check"]; ok {
		checkID := check.(map[string]any)["id"]
		funcs["last_result"] = func() any {
			if ctx.cache == nil {
				ctx.cache = make(map[string]any)
			}
			if result, ok := ctx.cache["last_result"]; ok {
				return result
			}
			status := map[string]any{
				"status":    "",
				"invalid":   false,
				"createdAt": nil,
				"duration":  0,
				"message":   "",
				"error":     "",
				"results":   make(map[string]any),
			}

			if checkID == "" {
				return status
			}

			if ctx.DB() == nil {
				logger.Errorf("[last_result] db connection not initialized")
				return status
			}

			type CheckStatus struct {
				Status    bool      `json:"status"`
				Invalid   bool      `json:"invalid,omitempty"`
				Time      string    `json:"time" gorm:"primaryKey"`
				Duration  int       `json:"duration"`
				Message   string    `json:"message,omitempty"`
				Error     string    `json:"error,omitempty"`
				Details   string    `json:"details" gorm:"details"`
				CreatedAt time.Time `json:"created_at,omitempty"`
			}

			var checkStatus CheckStatus
			err := ctx.DB().
				Table("check_statuses").
				Select("status", "invalid", "time", "duration", "message", "error", "details", "created_at").
				Where("check_id = ?", checkID).
				Order("time DESC").Limit(1).Scan(&checkStatus).Error
			if err != nil {
				logger.Warnf("[last_result] failed => %s", err)
				return status
			}

			status = map[string]any{
				"status":    checkStatus.Status,
				"invalid":   checkStatus.Invalid,
				"createdAt": checkStatus.CreatedAt,
				"duration":  checkStatus.Duration,
				"message":   checkStatus.Message,
				"error":     checkStatus.Error,
				"results":   make(map[string]any),
			}

			if checkStatus.Details != "" {
				var details = make(map[string]any)
				if err := json.Unmarshal([]byte(checkStatus.Details), &details); err == nil {
					status["results"] = details
				} else {
					if ctx.IsTrace() {
						ctx.Warnf("[last_result] Failed to unmarshal results: %s", err.Error())
					}
				}
			}
			ctx.cache["last_result"] = status
			return status
		}
	}
	return funcs
}

var CelFuncs []cel.EnvOption

func gcpIncidentToCheckResult(fnName string) cel.EnvOption {

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

	f := func(in any) map[string]any {
		var obj map[string]any
		switch v := in.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &obj); err != nil {
				return nil
			}
		case map[string]any:
			obj = v
		default:
			return nil
		}

		inc, ok := obj["incident"].(map[string]any)
		if !ok {
			return map[string]any{}
		}

		checkResult := map[string]any{
			"id":          uuid.NewSHA1(uuid.NameSpaceOID, []byte(inc["incident_id"].(string))).String(),
			"name":        fmt.Sprintf("[%s] %s", inc["incident_id"], inc["summary"]),
			"pass":        fmt.Sprint(inc["state"]) == "closed",
			"detail":      inc,
			"description": inc["summary"],
			"message":     fmt.Sprintf("[%s] %s", inc["incident_id"], inc["summary"]),
		}
		return checkResult
	}

	return cel.Function(fnName,
		cel.Overload(fnName+"_overload",
			[]*cel.Type{cel.AnyType},
			cel.AnyType,
			cel.UnaryBinding(func(obj ref.Val) ref.Val {
				return types.NewDynamicMap(types.DefaultTypeAdapter, f(obj.Value()))
			}),
		),
	)
}

func init() {
	CelFuncs = append(CelFuncs,
		gcpIncidentToCheckResult("gcp.incidents.toCheckResult"),
	)
}

package canary

import (
	"fmt"
	"strings"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/query"
	dutyTypes "github.com/flanksource/duty/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func UpdateCanaryStatusAndEvent(ctx context.Context, canary v1.Canary, results []*pkg.CheckResult) {
	if CanaryStatusChannel == nil {
		return
	}

	// Skip function if canary is not sourced from Kubernetes CRD
	if !strings.HasPrefix(canary.Annotations["source"], "kubernetes") {
		return
	}

	var checkStatus = make(map[string]*v1.CheckStatus)
	var duration int64
	var messages, errors []string
	var failEvents []string
	var msg, errorMsg string
	var pass = true
	var lastTransitionedTime *metav1.Time
	var highestLatency float64
	var uptimeAgg dutyTypes.Uptime

	transitioned := false
	for _, result := range results {
		// Increment duration
		duration += result.Duration

		// Set uptime and latency
		uptime, latency := metrics.Record(ctx, canary, result)
		checkID := canary.Status.Checks[result.Check.GetName()]
		checkStatus[checkID] = &v1.CheckStatus{
			Uptime1H:  uptime.String(),
			Latency1H: latency.String(),
		}

		// Increment aggregate uptime
		uptimeAgg.Passed += uptime.Passed
		uptimeAgg.Failed += uptime.Failed

		// Use highest latency for canary status
		if latency.Rolling1H > highestLatency {
			highestLatency = latency.Rolling1H
		}

		// Transition
		q := query.CheckQueryParams{Check: checkID, StatusCount: 1}
		if canary.Status.LastTransitionedTime != nil {
			q.Start = canary.Status.LastTransitionedTime.Format(time.RFC3339)
		}

		latestCheckStatus, err := db.LatestCheckStatus(ctx, checkID)
		if err != nil || latestCheckStatus == nil {
			transitioned = true
		} else if latestCheckStatus.Status != result.Pass {
			transitioned = true
		}
		if transitioned {
			transitionTime := time.Now()
			if latestCheckStatus != nil {
				transitionTime = latestCheckStatus.CreatedAt
			}

			checkStatus[checkID].LastTransitionedTime = &metav1.Time{Time: transitionTime}
			lastTransitionedTime = &metav1.Time{Time: transitionTime}
		}

		// Update status message
		if len(messages) == 1 {
			msg = messages[0]
		} else if len(messages) > 1 {
			msg = fmt.Sprintf("%s, (%d more)", messages[0], len(messages)-1)
		}
		if len(errors) == 1 {
			errorMsg = errors[0]
		} else if len(errors) > 1 {
			errorMsg = fmt.Sprintf("%s, (%d more)", errors[0], len(errors)-1)
		}

		if !result.Pass {
			failEvents = append(failEvents, fmt.Sprintf("%s-%s: %s", result.Check.GetType(), result.Check.GetEndpoint(), result.Message))
			pass = false
		}
	}

	payload := CanaryStatusPayload{
		Pass:                 pass,
		CheckStatus:          checkStatus,
		FailEvents:           failEvents,
		LastTransitionedTime: lastTransitionedTime,
		Message:              msg,
		ErrorMessage:         errorMsg,
		Uptime:               uptimeAgg.String(),
		Latency:              utils.Age(time.Duration(highestLatency) * time.Millisecond),
		NamespacedName:       canary.GetNamespacedName(),
	}

	CanaryStatusChannel <- payload
}

type CanaryStatusPayload struct {
	Pass                 bool
	CheckStatus          map[string]*v1.CheckStatus
	FailEvents           []string
	LastTransitionedTime *metav1.Time
	Message              string
	ErrorMessage         string
	Uptime               string
	Latency              string
	NamespacedName       types.NamespacedName
}

package changes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	changes "github.com/flanksource/changehub/api/v1"
	"github.com/flanksource/commons/logger"
)

func Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		logger.Errorf("%v method on /changes endpoint is not allowed", req.Method)
		fmt.Fprintf(w, "%v method not allowed", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	queryParams := req.URL.Query()
	timeString := queryParams.Get("since")
	if timeString == "" {
		logger.Errorf("since is a required parameter")
		fmt.Fprintf(w, "since is a required parameter")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	timeDuration, err := time.ParseDuration(timeString)
	if err != nil {
		logger.Errorf("since value not a valid duration")
		fmt.Fprintf(w, "since value not a valid duration")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	startTime := time.Now().UTC()

	results := []changes.Changes{}

	for _, check := range cache.Cache.Checks {
		i := 0
		scope := []changes.Scope{
			{
				Type:       "Namespace",
				Identifier: changes.Identifier{Name: check.GetNamespace()},
			},
			{
				Type:       "Name",
				Identifier: changes.Identifier{Name: check.GetName()},
			},
		}
		for key, value := range check.Labels {
			scope = append(scope, changes.Scope{
				Type: "Label",
				Identifier: changes.Identifier{
					Id:   key,
					Name: value,
				},
			})
		}
		changeResult := changes.Changes{
			FirstTimestamp: time.Time{},
			LastTimestamp:  time.Time{},
			Count:          0,
			Scope:          scope,
			Affects:        nil,
			Changes:        []changes.Change{},
		}
		var prevStatus bool
		for {
			if i >= len(check.Statuses) {
				break
			}
			checkTime, err := time.Parse(time.RFC3339, check.Statuses[i].Time)
			if err != nil {
				logger.Errorf("error parsing check records")
				fmt.Fprintf(w, "error parsing check records")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if checkTime.Add(timeDuration).Before(startTime) {
				break
			}
			if i == 0 {
				prevStatus = check.Statuses[i].Status
			} else {
				if check.Statuses[i].Status != prevStatus {
					category := "Succeeded"
					if !check.Statuses[i].Status {
						category = "Failed"
					}
					changeDetail := changes.Change{
						Type:        "Canary",
						Category:    category,
						Description: check.Description,
						Icon:        "",
						Data:        check.Statuses[i].Message,
						Identifier: changes.Identifier{
							Id: check.Key,
						},
					}
					if changeResult.Count == 0 {
						changeResult.LastTimestamp = checkTime
					}
					changeResult.FirstTimestamp = checkTime
					changeResult.Count++
					changeResult.Changes = append(changeResult.Changes, changeDetail)
				}

				prevStatus = check.Statuses[i].Status
			}
			i++
		}
		if changeResult.Count > 0 {
			results = append(results, changeResult)
		}
	}
	jsonData, err := json.Marshal(results)
	if err != nil {
		logger.Errorf("Failed to marshal data: %v", err)
		fmt.Fprintf(w, "{\"error\": \"internal\"}")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	changes "github.com/flanksource/changehub/api/v1"
	"github.com/flanksource/commons/logger"
)

func Changes(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		errorResonse(w, fmt.Errorf("unsupported method %s", req.Method), http.StatusMethodNotAllowed)
		return
	}
	q, err := cache.ParseQuery(req)
	if err != nil {
		errorResonse(w, err, http.StatusBadRequest)
		return
	}
	if q.Start == "" {
		q.Start = "1h"
	}

	results := []changes.Changes{}
	checks, err := cache.PostgresCache.Query(*q)
	if err != nil {
		errorResonse(w, err, http.StatusInternalServerError)
		return
	}

	for _, check := range checks {
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
		for i < len(check.Statuses) {
			checkTime, err := time.Parse(time.RFC3339, check.Statuses[i].Time)
			if err != nil {
				logger.Errorf("error parsing check records")
				fmt.Fprintf(w, "error parsing check records")
				w.WriteHeader(http.StatusInternalServerError)
				return
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
		errorResonse(w, errors.New("error marshalling json"), http.StatusInternalServerError)
		return
	}
	if _, err = w.Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
	}
}

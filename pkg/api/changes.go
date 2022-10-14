package api

import (
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	changes "github.com/flanksource/changehub/api/v1"
	"github.com/labstack/echo/v4"
)

func Changes(c echo.Context) error {
	q, err := cache.ParseQuery(c)
	if err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}
	if q.Start == "" {
		q.Start = "1h"
	}

	results := []changes.Changes{}
	checks, err := cache.PostgresCache.Query(*q)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	for _, check := range checks {
		i := 0
		scope := []changes.Scope{
			{
				Type:       "Namespace",
				Identifier: changes.Identifier{Name: check.Namespace},
			},
			{
				Type:       "Name",
				Identifier: changes.Identifier{Name: check.Name},
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
				return errorResonse(c, err, http.StatusInternalServerError)
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
							Id: check.ID.String(),
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
	return c.JSON(http.StatusOK, results)
}

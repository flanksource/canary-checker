package changes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
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

	changeResults := map[string][]pkg.CheckStatus{}
	for checkName, check := range cache.Cache.Checks {
		i := 0
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
					if changeResults[checkName] == nil {
						changeResults[checkName] = []pkg.CheckStatus{check.Statuses[i-1]}
					} else {
						changeResults[checkName] = append(changeResults[checkName], check.Statuses[i])
					}
				}
				prevStatus = check.Statuses[i].Status
			}
			i++
		}
	}
	jsonData, err := json.Marshal(changeResults)
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

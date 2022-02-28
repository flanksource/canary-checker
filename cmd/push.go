package cmd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/push"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
)

var name, description, checkType, status, duration, message, checkDetails, errorMessage string
var Push = &cobra.Command{
	Use:   "push <servers>",
	Short: "Push a check to multiple canary-checker servers",
	Run: func(cmd *cobra.Command, servers []string) {
		var passed bool
		var details []byte
		if strings.ToLower(status) == "passed" {
			passed = true
		}
		checkDuration, err := time.ParseDuration(duration)
		if err != nil {
			logger.Errorf("Invalid duration: %s", err)
		}
		if checkDetails != "" {
			detailsContent, err := ioutil.ReadFile(checkDetails)
			if err != nil {
				logger.Errorf("Failed to read check details: %s", err)
			} else {
				details, err = json.Marshal(detailsContent)
				if err != nil {
					logger.Errorf("Failed to marshal check details: %s", err)
				}
			}

		}
		for _, server := range servers {
			checkStatus := pkg.CheckStatus{
				Status:   passed,
				Duration: int(checkDuration.Milliseconds()),
				Time:     time.Now().UTC().Format(time.RFC3339),
				Message:  message,
				Detail:   details,
			}
			check := pkg.Check{
				Name:        name,
				Type:        checkType,
				Description: description,
			}
			data := push.QueueData{
				Check:  check,
				Status: checkStatus,
			}
			jsonData, err := json.Marshal(data)
			if err != nil {
				logger.Errorf("error unmarshalling request body: %v", err)
			}
			err = push.PostDataToServer(server, bytes.NewBuffer(jsonData))
			if err != nil {
				logger.Errorf("error pushing data to server: %v", err)
			}
		}
	},
}

func init() {
	Push.Flags().StringVarP(&name, "name", "n", "", "Name of the check")
	Push.Flags().StringVarP(&description, "description", "d", "", "Description for the check")
	Push.Flags().StringVarP(&checkType, "type", "t", "", "Type of the check")
	Push.Flags().StringVarP(&status, "status", "s", "", "Status of the check. Passed or failed")
	Push.Flags().StringVar(&duration, "duration", "", "Duration for the check. Follows golang duration format")
	Push.Flags().StringVarP(&message, "message", "m", "", "Message for the check")
	Push.Flags().StringVarP(&errorMessage, "error-message", "e", "", "Error message for the check")
	Push.Flags().StringVarP(&checkDetails, "details", "D", "", "file containing the check details")
}

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/commons/files"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func init() {
	check := pkg.Check{}
	status := pkg.CheckStatus{}
	var details string
	var checkID, checkCanaryID string
	var Push = &cobra.Command{
		Use:   "push <servers>",
		Short: "Push a check to multiple canary-checker servers",
		Run: func(cmd *cobra.Command, servers []string) {
			check.ID, _ = uuid.Parse(checkID)
			check.CanaryID, _ = uuid.Parse(checkCanaryID)

			if details != "" && files.Exists(details) {
				detailsContent, err := os.ReadFile(details)
				if err != nil {
					logger.Fatalf("Failed to read check details: %s", err)
				}
				status.Detail = string(detailsContent)
			}
			status.Time = time.Now().UTC().Format(time.RFC3339)
			for _, server := range servers {
				data := api.QueueData{
					Check:  check,
					Status: status,
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					logger.Errorf("error unmarshalling request body: %v", err)
				}
				err = api.PostDataToServer(server, bytes.NewBuffer(jsonData))
				if err != nil {
					logger.Errorf("error pushing data to server: %v", err)
				}
			}
		},
	}

	Push.Flags().StringVar(&checkID, "id", "", "UUID of check")
	Push.Flags().StringVar(&checkCanaryID, "parent-id", "", "UUID of parent canary")
	Push.Flags().StringVarP(&check.Name, "name", "n", "", "Name of check")
	Push.Flags().StringVarP(&check.Type, "type", "t", "", "Type of check")
	Push.Flags().StringVarP(&check.Description, "description", "d", "", "Description of check")
	Push.Flags().StringVar(&check.Namespace, "namespace", "default", "Name of canary")
	Push.Flags().StringVar(&status.Error, "error", "", "Error of check")
	Push.Flags().StringVar(&status.Message, "message", "", "Message of check")
	Push.Flags().StringVar(&details, "detail", "", "Detail of check")
	Push.Flags().Int64Var(&status.DurationMs, "duration", 0, "Duration of check in milliseconds")
	Push.Flags().BoolVar(&status.Status, "passed", true, "Passed status of check")
	Root.AddCommand(Push)
}

package canary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/upstream"
	"gorm.io/gorm/clause"
)

var UpstreamConf upstream.UpstreamConfig

var tablesToReconcile = []string{
	"checks",
	"check_statuses",
}

// SyncWithUpstream coordinates with upstream and pushes any resource
// that are missing on the upstream.
func SyncWithUpstream() {
	ctx := context.New(nil, nil, db.Gorm, v1.Canary{})

	jobHistory := models.NewJobHistory("SyncCanaryResultsWithUpstream", "Canary", "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	reconciler := upstream.NewUpstreamReconciler(UpstreamConf, 5)
	for _, table := range tablesToReconcile {
		if err := reconciler.Sync(ctx, table); err != nil {
			jobHistory.AddError(err.Error())
			logger.Errorf("failed to sync table %s: %v", table, err)
		} else {
			jobHistory.IncrSuccess()
		}
	}
}

func Pull() {
	jobHistory := models.NewJobHistory("PullAgentCanaries", "Canary", "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	if err := pull(UpstreamConf); err != nil {
		jobHistory.AddError(err.Error())
		logger.Errorf("Error pulling upstream: %v", err)
	}

	jobHistory.IncrSuccess()
}

func pull(config upstream.UpstreamConfig) error {
	endpoint, err := url.JoinPath(config.Host, "upstream", "canary", "pull", config.AgentName)
	if err != nil {
		return fmt.Errorf("error creating url endpoint for host %s: %w", config.Host, err)
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(config.Username, config.Password)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	var canaries []models.Canary
	if err := json.NewDecoder(resp.Body).Decode(&canaries); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if len(canaries) == 0 {
		return nil
	}

	return db.Gorm.Omit("agent_id").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&canaries).Error
}

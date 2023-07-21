package canary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"gorm.io/gorm/clause"
)

func Pull() {
	if err := pull(UpstreamConf); err != nil {
		logger.Errorf("Error pulling upstream: %v", err)
	}
}

func pull(config UpstreamConfig) error {
	endpoint, err := url.JoinPath(UpstreamConf.Host, "upstream", "canary", "pull", config.AgentName)
	if err != nil {
		return fmt.Errorf("error creating url endpoint for host %s: %w", config.Host, err)
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(UpstreamConf.Username, UpstreamConf.Password)

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

type PushData struct {
	AgentName     string               `json:"agent_name,omitempty"`
	Checks        []models.Check       `json:"checks,omitempty"`
	CheckStatuses []models.CheckStatus `json:"check_statuses,omitempty"`
}

func (t *PushData) Empty() bool {
	return len(t.Checks) == 0 && len(t.CheckStatuses) == 0
}

func pushToUpstream(data PushData) error {
	data.AgentName = UpstreamConf.AgentName
	payloadBuf := new(bytes.Buffer)
	if err := json.NewEncoder(payloadBuf).Encode(data); err != nil {
		return fmt.Errorf("error encoding msg: %w", err)
	}

	endpoint, err := url.JoinPath(UpstreamConf.Host, "upstream", "push")
	if err != nil {
		return fmt.Errorf("error creating url endpoint for host %s: %w", UpstreamConf.Host, err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, payloadBuf)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %w", err)
	}

	req.SetBasicAuth(UpstreamConf.Username, UpstreamConf.Password)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if !collections.Contains([]int{http.StatusOK, http.StatusCreated}, resp.StatusCode) {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream server returned error status[%d]: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

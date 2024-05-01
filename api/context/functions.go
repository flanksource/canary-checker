package context

import (
	"encoding/json"
	"time"

	"github.com/flanksource/commons/logger"
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
			if checkID == "" {
				return nil
			}

			if ctx.DB() == nil {
				logger.Errorf("[last_result] db connection not initialized")
				return nil
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
				return nil
			}

			lastResult := map[string]any{
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
					lastResult["results"] = details
				} else {
					if ctx.IsTrace() {
						ctx.Warnf("[last_result] Failed to unmarshal results: %s", err.Error())
					}
				}
			}
			ctx.cache["last_result"] = lastResult
			return lastResult
		}
	}
	return funcs
}

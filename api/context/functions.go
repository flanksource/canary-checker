package context

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/uuid"
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
			status := map[string]any{
				"status":    "",
				"invalid":   false,
				"createdAt": nil,
				"duration":  0,
				"message":   "",
				"error":     "",
				"results":   make(map[string]any),
			}

			if checkID == "" {
				return status
			}

			if ctx.DB() == nil {
				logger.Errorf("[last_result] db connection not initialized")
				return status
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
				return status
			}

			status = map[string]any{
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
					status["results"] = details
				} else {
					if ctx.IsTrace() {
						ctx.Warnf("[last_result] Failed to unmarshal results: %s", err.Error())
					}
				}
			}
			ctx.cache["last_result"] = status
			return status
		}
	}
	return funcs
}

var CelFuncs []cel.EnvOption

func gcpIncidentToCheckResult(fnName string) cel.EnvOption {
	f := func(in any) map[string]any {
		var obj map[string]any
		switch v := in.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &obj); err != nil {
				return nil
			}
		case map[string]any:
			obj = v
		default:
			return nil
		}

		inc, ok := obj["incident"].(map[string]any)
		if !ok {
			return map[string]any{}
		}

		checkResult := map[string]any{
			"id":          uuid.NewSHA1(uuid.NameSpaceOID, []byte(inc["incident_id"].(string))).String(),
			"name":        fmt.Sprintf("[%s] %s", inc["incident_id"], inc["summary"]),
			"pass":        fmt.Sprint(inc["state"]) == "closed",
			"detail":      inc,
			"description": inc["summary"],
			"message":     fmt.Sprintf("[%s] %s", inc["incident_id"], inc["summary"]),
		}
		return checkResult
	}

	return cel.Function(fnName,
		cel.Overload(fnName+"_overload",
			[]*cel.Type{cel.AnyType},
			cel.AnyType,
			cel.UnaryBinding(func(obj ref.Val) ref.Val {
				return types.NewDynamicMap(types.DefaultTypeAdapter, f(obj.Value()))
			}),
		),
	)
}

func init() {
	CelFuncs = append(CelFuncs,
		gcpIncidentToCheckResult("gcp.incidents.toCheckResult"),
	)
}

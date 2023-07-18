package api

import (
	"net/http"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty/models"
	"github.com/labstack/echo/v4"
)

// Pull returns all canaries for the requested agent
func Pull(c echo.Context) error {
	agentName := c.Param("agent_name")

	var canaries []models.Canary
	if err := db.Gorm.Where("deleted_at IS NULL").Joins("LEFT JOIN agents ON canaries.agent_id = agents.id").Where("agents.name = ?", agentName).Find(&canaries).Error; err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, canaries)
}

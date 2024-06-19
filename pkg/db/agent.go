package db

import (
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
)

func GetAgent(ctx context.Context, name string) (models.Agent, error) {
	var agent models.Agent
	err := ctx.DB().Where("name = ? AND deleted_at IS NULL", name).First(&agent).Error
	return agent, err
}

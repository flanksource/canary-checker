package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

// Store entry in config_component_relationship table
type ConfigComponentRelationship struct {
	ComponentID uuid.UUID
	ConfigID    uuid.UUID
	SelectorID  string
	DeletedAt   *time.Time
}

func configQuery(config pkg.Config) *gorm.DB {
	query := Gorm.Table("config_items")
	if config.ConfigType != "" {
		query = query.Where("config_type = ?", config.ConfigType)
	}
	if config.Name != "" {
		query = query.Where("name = ?", config.Name)
	}
	if config.Namespace != "" {
		query = query.Where("namespace = ?", config.Namespace)
	}

	if config.Labels != nil && len(config.Labels) > 0 {
		query = query.Where("tags @> ?", config.Labels)
	}

	// ExternalType is derived from v1.Config.Type which is a user input field
	// It can refer to both external_type or config_type for now
	if config.ExternalType != "" {
		query = query.Where("external_type = @external_type OR config_type = @external_type", sql.Named("external_type", config.ExternalType))
	}
	if len(config.ExternalID) > 0 {
		query = query.Where("external_id @> ?", config.ExternalID)
	}
	return query
}

func FindConfig(config pkg.Config) (pkg.Config, error) {
	if Gorm == nil {
		logger.Debugf("Config lookup on %v will be ignored, db not initialized", config)
		return pkg.Config{}, gorm.ErrRecordNotFound
	}
	var dbConfigObject pkg.Config
	query := configQuery(config)
	err := query.First(&dbConfigObject).Error
	return dbConfigObject, err
}

func PersistConfigComponentRelationship(configID, componentID uuid.UUID, selectorID string) error {
	relationship := ConfigComponentRelationship{
		ComponentID: componentID,
		ConfigID:    configID,
		SelectorID:  selectorID,
		DeletedAt:   nil,
	}
	return Gorm.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "config_id"}, {Name: "component_id"}},
		UpdateAll: true,
	}).Create(&relationship).Error
}

func DeleteConfigRelationshipForComponent(componentID uuid.UUID, deleteTime time.Time) error {
	return Gorm.Model(&ConfigComponentRelationship{}).Where("component_id = ?", componentID).Update("deleted_at", deleteTime).Error
}

func GetConfigRelationshipsForComponent(componentID uuid.UUID) ([]ConfigComponentRelationship, error) {
	var relationships []ConfigComponentRelationship
	if err := Gorm.Where("component_id = ? AND deleted_at IS NOT NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
}

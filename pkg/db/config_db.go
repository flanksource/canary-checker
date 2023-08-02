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
	query := Gorm.Table("config_items").Where("agent_id = '00000000-0000-0000-0000-000000000000'")
	if config.ConfigClass != "" {
		query = query.Where("config_class = ?", config.ConfigClass)
	}
	if config.Name != "" {
		query = query.Where("name = ?", config.Name)
	}
	if config.Namespace != "" {
		query = query.Where("namespace = ?", config.Namespace)
	}

	if config.Tags != nil && len(config.Tags) > 0 {
		query = query.Where("tags @> ?", config.Tags)
	}

	// Type is derived from v1.Config.Type which is a user input field
	// It can refer to both type or config_class for now
	if config.Type != "" {
		query = query.Where("type = @config_type OR config_class = @config_type", sql.Named("config_type", config.Type))
	}
	if len(config.ExternalID) > 0 {
		query = query.Where("external_id @> ?", config.ExternalID)
	}
	return query
}

func FindConfig(config pkg.Config) (*pkg.Config, error) {
	if Gorm == nil {
		logger.Debugf("Config lookup on %v will be ignored, db not initialized", config)
		return nil, gorm.ErrRecordNotFound
	}
	var dbConfigObject pkg.Config
	query := configQuery(config)
	tx := query.Limit(1).Find(&dbConfigObject)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}
	return &dbConfigObject, nil
}

func FindConfigForComponent(componentID, configType string) ([]pkg.Config, error) {
	var dbConfigObjects []pkg.Config
	relationshipQuery := Gorm.Table("config_component_relationships").
		Select("config_id").
		Where("component_id = ? AND deleted_at IS NULL", componentID)
	query := Gorm.Table("config_items").Where("id IN (?)", relationshipQuery)
	if configType != "" {
		query = query.Where("type = @config_type OR config_class = @config_type", sql.Named("config_type", configType))
	}
	err := query.Find(&dbConfigObjects).Error
	return dbConfigObjects, err
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
	if err := Gorm.Where("component_id = ? AND deleted_at IS NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
}

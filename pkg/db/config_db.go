package db

import (
	"database/sql"
	"time"

	"github.com/flanksource/commons/collections"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/flanksource/canary-checker/pkg"
)

// Store entry in config_component_relationship table
type configComponentRelationship struct {
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
	var dbConfigObject pkg.Config
	query := configQuery(config)
	err := query.First(&dbConfigObject).Error
	return dbConfigObject, err
}

func PersistConfigComponentRelationship(configID, componentID uuid.UUID, selectorID string) error {
	relationship := configComponentRelationship{
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
	return Gorm.Model(&configComponentRelationship{}).Where("component_id = ?", componentID).Update("deleted_at", deleteTime).Error
}

func GetConfigRelationshipsForComponent(componentID uuid.UUID) ([]configComponentRelationship, error) {
	var relationships []configComponentRelationship
	if err := Gorm.Where("component_id = ? AND deleted_at IS NOT NULL", componentID).Find(&relationships).Error; err != nil {
		return relationships, err
	}
	return relationships, nil
}

func UpsertComponentConfigRelationship(componentID uuid.UUID, configs pkg.Configs) error {
	if len(configs) == 0 {
		return nil
	}

	var selectorIDs []string
	var configIDs []string
	relationships, err := GetConfigRelationshipsForComponent(componentID)
	if err != nil {
		return err
	}

	for _, r := range relationships {
		selectorIDs = append(selectorIDs, r.SelectorID)
		configIDs = append(configIDs, r.ConfigID.String())
	}

	for _, config := range configs {
		dbConfig, err := FindConfig(*config)
		if err != nil {
			return errors.Wrap(err, "error fetching config from database")
		}
		selectorID := dbConfig.GetSelectorID()

		// If selectorID already exists, no action is required
		if collections.Contains(selectorIDs, selectorID) {
			continue
		}

		// If configID does not exist, create a new relationship
		if !collections.Contains(configIDs, dbConfig.ID.String()) {
			if err := PersistConfigComponentRelationship(dbConfig.ID, componentID, selectorID); err != nil {
				return errors.Wrap(err, "error persisting config relationships")
			}
			continue
		}

		// If config_id exists mark old row as deleted and update selector_id
		if err := Gorm.Model(&configComponentRelationship{}).Where("component_id = ? AND config_id = ?", componentID, dbConfig.ID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return errors.Wrap(err, "error updating config relationships")
		}
		if err := PersistConfigComponentRelationship(config.ID, componentID, selectorID); err != nil {
			return errors.Wrap(err, "error persisting config relationships")
		}
	}
	return nil
}

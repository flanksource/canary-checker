package db

import (
	"database/sql"

	"github.com/flanksource/canary-checker/pkg"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
		query = query.Where("external_id @> ?", pq.StringArray(config.ExternalID))
	}
	return query
}

func FetchConfig(config pkg.Config) (pkg.Config, error) {
	var dbConfigObject pkg.Config
	query := configQuery(config)
	err := query.First(&dbConfigObject).Error
	return dbConfigObject, err
}

func PersistConfigComponentRelationship(configID uuid.UUID, componentID uuid.UUID) error {
	// Store entry in config_component_relationship table
	type configComponentRelationship struct {
		ComponentID uuid.UUID
		ConfigID    uuid.UUID
	}

	relationship := configComponentRelationship{ComponentID: componentID, ConfigID: configID}
	return Gorm.Clauses(clause.OnConflict{DoNothing: true}).Create(&relationship).Error
}

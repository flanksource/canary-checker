package db

import (
	"github.com/flanksource/canary-checker/pkg"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func configQuery(config pkg.Config) *gorm.DB {
	query := Gorm.Table("config_item")

	if config.ConfigType != "" {
		query = query.Where("config_type = ?", config.ConfigType)
	}
	if config.Name != "" {
		query = query.Where("name = ?", config.Name)
	}
	if config.ExternalType != "" {
		query = query.Where("external_type = ?", config.ExternalType)
	}
	if len(config.ExternalId) > 0 {
		query = query.Where("external_id = ?", pq.StringArray(config.ExternalId))
	}
	return query
}

func FetchConfig(config pkg.Config) (string, error) {
	var configSpec string
	query := configQuery(config)
	err := query.Select("config").Find(&config).Error
	return configSpec, err
}

func PersistConfigComponentRelationship(config pkg.Config, componentID uuid.UUID) error {
	var configID uuid.UUID
	query := Gorm.Table("config_item").Select("id")

	if config.ConfigType != "" {
		query = query.Where("config_type = ?", config.ConfigType)
	}
	if config.Name != "" {
		query = query.Where("name = ?", config.Name)
	}
	if config.ExternalType != "" {
		query = query.Where("external_type = ?", config.ExternalType)
	}
	if len(config.ExternalId) > 0 {
		query = query.Where("external_id = ?", pq.StringArray(config.ExternalId))
	}

	err := query.Find(&configID).Error
	if err != nil {
		return err
	}

	// Store entry in config_component_relationship table
	type configComponentRelationship struct {
		componentID uuid.UUID
		configID    uuid.UUID
	}

	relationship := configComponentRelationship{componentID: componentID, configID: configID}
	return Gorm.Clauses(clause.OnConflict{DoNothing: true}).Create(&relationship).Error
}

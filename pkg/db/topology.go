package db

import (
	"database/sql"
	"encoding/json"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/models"
	"github.com/flanksource/commons/logger"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	. "github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func NewSystemModel(system *pkg.System) models.System {
	return models.System{
		Name:       system.Name,
		ExternalID: system.Id,
		Text:       null.StringFrom(system.Text),
		Icon:       null.StringFrom(system.Icon),
		Labels:     mapToJson(system.Labels),
		Owner:      null.StringFrom(system.Owner),
		Tooltip:    null.StringFrom(system.Tooltip),
		Status:     system.Status,
		Properties: null.JSONFrom(system.Properties.AsJSON()),
		Type:       null.StringFrom(system.Type),
	}
}

func NewComponentModel(component *pkg.Component) models.Component {
	return models.Component{
		ExternalID: component.GetID(),
		Name:       component.Name,
		Status:     component.Status,
		Labels:     mapToJson(component.Labels),
		Text:       null.StringFrom(component.Text),
		Icon:       null.StringFrom(component.Icon),
		Owner:      null.StringFrom(component.Owner),
		Tooltip:    null.StringFrom(component.Tooltip),
		Properties: null.JSONFrom(component.Properties.AsJSON()),
		Type:       null.StringFrom(component.Type),
	}
}

func FindSystem(systemId, systemType string) (*models.System, error) {
	if sys, err := models.Systems(Where("external_id = ? AND type = ?", systemId, systemType)).OneG(); err == nil {
		return sys, nil
	} else if err == sql.ErrNoRows {
		return nil, nil
	} else {
		return nil, err
	}
}

func AddSystemSpec(id string, system v1.System) (string, error) {
	spec, err := json.Marshal(system)
	if err != nil {
		return "", err
	}

	existing, err := FindSystem(id, system.Spec.Type)
	if err != nil {
		return "", err
	}

	_system := models.System{
		Name:       system.Name,
		ExternalID: id,
		Type:       null.StringFrom(system.Spec.Type),
		Spec:       null.JSONFrom(spec),
	}

	if existing == nil {
		if err := _system.InsertG(boil.Infer()); err != nil {
			return "", err
		}
	} else {
		_system.ID = existing.ID
		if _, err := _system.UpdateG(getColumnsFromString("name", "spec")); err != nil {
			return "", err
		}
	}
	return _system.ID, nil
}

func AddSystem(system *pkg.System, cols ...string) (string, error) {
	_system := NewSystemModel(system)
	existing, err := models.Systems(Where("external_id = ? AND type = ?", system.Id, system.Type)).OneG()
	if err == sql.ErrNoRows {
		if err := _system.InsertG(boil.Infer()); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		_system.ID = existing.ID
		if _, err := _system.UpdateG(getColumnsFromString(cols...)); err != nil {
			return "", err
		}
	}
	return _system.ID, nil
}

var componentUpdate = []string{"name", "status", "description", "labels", "text", "icon", "owner", "tooltip", "properties", "type"}

func PersistComponent(systemId string, component *pkg.Component, parent *models.Component) error {
	_component := NewComponentModel(component)
	_component.SystemID = null.StringFrom(systemId)
	if parent != nil {
		_component.ParentID = null.StringFrom(parent.ID)
	}

	existing, err := models.Components(Where("system_id = ? AND external_id = ? AND type = ?", systemId, _component.ExternalID, component.Type)).OneG()
	logger.Debugf("Inserting %s id=%s type=%s external_id=%s) ", component, systemId, component.Type, _component.ExternalID)

	if err == sql.ErrNoRows {
		if err := _component.InsertG(boil.Infer()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		_component.ID = existing.ID
		logger.Debugf("Update %s (%s=%s)", component, existing.ID, component.GetID())
		if _, err := _component.UpdateG(getColumnsFromString(componentUpdate...)); err != nil {
			return err
		}
	}
	for _, child := range component.Components {
		if err := PersistComponent(systemId, child, &_component); err != nil {
			return err
		}
	}
	return nil
}

func Persist(results []*pkg.System) error {
	for _, system := range results {
		id, err := AddSystem(system)
		if err != nil {
			return err
		}
		for _, component := range system.Components {
			if err := PersistComponent(id, component, nil); err != nil {
				return nil
			}
		}
	}
	return nil
}

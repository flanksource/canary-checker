package db

import (
	"database/sql"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	. "github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func NewSystemModel(system *pkg.System) models.System {
	return models.System{
		Name:       system.Name,
		ExternalID: system.Id,
		Icon:       null.StringFrom(system.Icon),
		Owner:      null.StringFrom(system.Owner),
		Tooltip:    null.StringFrom(system.Tooltip),
		Status:     system.Status,
		Properties: null.JSONFrom(system.Properties.AsJSON()),
		Type:       null.StringFrom(system.Type),
	}

}

func NewComponentModel(component *pkg.Component) models.Component {
	return models.Component{
		ExternalID: component.Id,
		Name:       component.Name,
		Status:     component.Status,
		Icon:       null.StringFrom(component.Icon),
		Owner:      null.StringFrom(component.Owner),
		Tooltip:    null.StringFrom(component.Tooltip),
		Properties: null.JSONFrom(component.Properties.AsJSON()),
		Type:       null.StringFrom(component.Type),
	}
}

func PersistComponent(system models.System, component *pkg.Component, parent *models.Component) error {
	_component := NewComponentModel(component)
	_component.SystemID = null.StringFrom(system.ID)
	if parent != nil {
		_component.ParentID = null.StringFrom(parent.ID)
	}

	existing, err := models.Components(Where("system_id = ? AND external_id = ? AND type = ?", system.ID, component.Id, component.Type)).OneG()
	if err == sql.ErrNoRows {
		if err := _component.InsertG(boil.Infer()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		_component.ID = existing.ID
		if _, err := _component.UpdateG(boil.Infer()); err != nil {
			return err
		}
	}
	for _, child := range component.Components {
		if err := PersistComponent(system, child, &_component); err != nil {
			return err
		}
	}
	return nil
}

func Persist(results []*pkg.System) error {
	for _, system := range results {
		_system := NewSystemModel(system)
		existing, err := models.Systems(Where("external_id = ? AND type = ?", system.Id, system.Type)).OneG()
		if err == sql.ErrNoRows {
			if err := _system.InsertG(boil.Infer()); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			_system.ID = existing.ID
			if _, err := _system.UpdateG(boil.Infer()); err != nil {
				return err
			}
		}
		for _, component := range system.Components {
			if err := PersistComponent(_system, component, nil); err != nil {
				return nil
			}

		}
	}
	return nil
}

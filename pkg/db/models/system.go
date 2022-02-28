// Code generated by SQLBoiler 4.8.3 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/friendsofgo/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/queries/qmhelper"
	"github.com/volatiletech/strmangle"
)

// System is an object representing the database table.
type System struct {
	ID         string      `boil:"id" json:"id" toml:"id" yaml:"id"`
	ExternalID string      `boil:"external_id" json:"external_id" toml:"external_id" yaml:"external_id"`
	Name       string      `boil:"name" json:"name" toml:"name" yaml:"name"`
	Text       null.String `boil:"text" json:"text,omitempty" toml:"text" yaml:"text,omitempty"`
	Status     string      `boil:"status" json:"status" toml:"status" yaml:"status"`
	Hidden     bool        `boil:"hidden" json:"hidden" toml:"hidden" yaml:"hidden"`
	Silenced   bool        `boil:"silenced" json:"silenced" toml:"silenced" yaml:"silenced"`
	Labels     null.JSON   `boil:"labels" json:"labels,omitempty" toml:"labels" yaml:"labels,omitempty"`
	Tooltip    null.String `boil:"tooltip" json:"tooltip,omitempty" toml:"tooltip" yaml:"tooltip,omitempty"`
	Lifecycle  null.String `boil:"lifecycle" json:"lifecycle,omitempty" toml:"lifecycle" yaml:"lifecycle,omitempty"`
	Icon       null.String `boil:"icon" json:"icon,omitempty" toml:"icon" yaml:"icon,omitempty"`
	Owner      null.String `boil:"owner" json:"owner,omitempty" toml:"owner" yaml:"owner,omitempty"`
	Type       null.String `boil:"type" json:"type,omitempty" toml:"type" yaml:"type,omitempty"`
	Properties null.JSON   `boil:"properties" json:"properties,omitempty" toml:"properties" yaml:"properties,omitempty"`
	Spec       null.JSON   `boil:"spec" json:"spec,omitempty" toml:"spec" yaml:"spec,omitempty"`
	CreatedAt  time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt  time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`

	R *systemR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L systemL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var SystemColumns = struct {
	ID         string
	ExternalID string
	Name       string
	Text       string
	Status     string
	Hidden     string
	Silenced   string
	Labels     string
	Tooltip    string
	Lifecycle  string
	Icon       string
	Owner      string
	Type       string
	Properties string
	Spec       string
	CreatedAt  string
	UpdatedAt  string
}{
	ID:         "id",
	ExternalID: "external_id",
	Name:       "name",
	Text:       "text",
	Status:     "status",
	Hidden:     "hidden",
	Silenced:   "silenced",
	Labels:     "labels",
	Tooltip:    "tooltip",
	Lifecycle:  "lifecycle",
	Icon:       "icon",
	Owner:      "owner",
	Type:       "type",
	Properties: "properties",
	Spec:       "spec",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

var SystemTableColumns = struct {
	ID         string
	ExternalID string
	Name       string
	Text       string
	Status     string
	Hidden     string
	Silenced   string
	Labels     string
	Tooltip    string
	Lifecycle  string
	Icon       string
	Owner      string
	Type       string
	Properties string
	Spec       string
	CreatedAt  string
	UpdatedAt  string
}{
	ID:         "system.id",
	ExternalID: "system.external_id",
	Name:       "system.name",
	Text:       "system.text",
	Status:     "system.status",
	Hidden:     "system.hidden",
	Silenced:   "system.silenced",
	Labels:     "system.labels",
	Tooltip:    "system.tooltip",
	Lifecycle:  "system.lifecycle",
	Icon:       "system.icon",
	Owner:      "system.owner",
	Type:       "system.type",
	Properties: "system.properties",
	Spec:       "system.spec",
	CreatedAt:  "system.created_at",
	UpdatedAt:  "system.updated_at",
}

// Generated where

var SystemWhere = struct {
	ID         whereHelperstring
	ExternalID whereHelperstring
	Name       whereHelperstring
	Text       whereHelpernull_String
	Status     whereHelperstring
	Hidden     whereHelperbool
	Silenced   whereHelperbool
	Labels     whereHelpernull_JSON
	Tooltip    whereHelpernull_String
	Lifecycle  whereHelpernull_String
	Icon       whereHelpernull_String
	Owner      whereHelpernull_String
	Type       whereHelpernull_String
	Properties whereHelpernull_JSON
	Spec       whereHelpernull_JSON
	CreatedAt  whereHelpertime_Time
	UpdatedAt  whereHelpertime_Time
}{
	ID:         whereHelperstring{field: "\"system\".\"id\""},
	ExternalID: whereHelperstring{field: "\"system\".\"external_id\""},
	Name:       whereHelperstring{field: "\"system\".\"name\""},
	Text:       whereHelpernull_String{field: "\"system\".\"text\""},
	Status:     whereHelperstring{field: "\"system\".\"status\""},
	Hidden:     whereHelperbool{field: "\"system\".\"hidden\""},
	Silenced:   whereHelperbool{field: "\"system\".\"silenced\""},
	Labels:     whereHelpernull_JSON{field: "\"system\".\"labels\""},
	Tooltip:    whereHelpernull_String{field: "\"system\".\"tooltip\""},
	Lifecycle:  whereHelpernull_String{field: "\"system\".\"lifecycle\""},
	Icon:       whereHelpernull_String{field: "\"system\".\"icon\""},
	Owner:      whereHelpernull_String{field: "\"system\".\"owner\""},
	Type:       whereHelpernull_String{field: "\"system\".\"type\""},
	Properties: whereHelpernull_JSON{field: "\"system\".\"properties\""},
	Spec:       whereHelpernull_JSON{field: "\"system\".\"spec\""},
	CreatedAt:  whereHelpertime_Time{field: "\"system\".\"created_at\""},
	UpdatedAt:  whereHelpertime_Time{field: "\"system\".\"updated_at\""},
}

// SystemRels is where relationship names are stored.
var SystemRels = struct {
	Components string
}{
	Components: "Components",
}

// systemR is where relationships are stored.
type systemR struct {
	Components ComponentSlice `boil:"Components" json:"Components" toml:"Components" yaml:"Components"`
}

// NewStruct creates a new relationship struct
func (*systemR) NewStruct() *systemR {
	return &systemR{}
}

// systemL is where Load methods for each relationship are stored.
type systemL struct{}

var (
	systemAllColumns            = []string{"id", "external_id", "name", "text", "status", "hidden", "silenced", "labels", "tooltip", "lifecycle", "icon", "owner", "type", "properties", "spec", "created_at", "updated_at"}
	systemColumnsWithoutDefault = []string{"external_id", "name", "text", "status", "labels", "tooltip", "lifecycle", "icon", "owner", "type", "properties", "spec"}
	systemColumnsWithDefault    = []string{"id", "hidden", "silenced", "created_at", "updated_at"}
	systemPrimaryKeyColumns     = []string{"id"}
)

type (
	// SystemSlice is an alias for a slice of pointers to System.
	// This should almost always be used instead of []System.
	SystemSlice []*System
	// SystemHook is the signature for custom System hook methods
	SystemHook func(boil.Executor, *System) error

	systemQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	systemType                 = reflect.TypeOf(&System{})
	systemMapping              = queries.MakeStructMapping(systemType)
	systemPrimaryKeyMapping, _ = queries.BindMapping(systemType, systemMapping, systemPrimaryKeyColumns)
	systemInsertCacheMut       sync.RWMutex
	systemInsertCache          = make(map[string]insertCache)
	systemUpdateCacheMut       sync.RWMutex
	systemUpdateCache          = make(map[string]updateCache)
	systemUpsertCacheMut       sync.RWMutex
	systemUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

var systemBeforeInsertHooks []SystemHook
var systemBeforeUpdateHooks []SystemHook
var systemBeforeDeleteHooks []SystemHook
var systemBeforeUpsertHooks []SystemHook

var systemAfterInsertHooks []SystemHook
var systemAfterSelectHooks []SystemHook
var systemAfterUpdateHooks []SystemHook
var systemAfterDeleteHooks []SystemHook
var systemAfterUpsertHooks []SystemHook

// doBeforeInsertHooks executes all "before insert" hooks.
func (o *System) doBeforeInsertHooks(exec boil.Executor) (err error) {
	for _, hook := range systemBeforeInsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpdateHooks executes all "before Update" hooks.
func (o *System) doBeforeUpdateHooks(exec boil.Executor) (err error) {
	for _, hook := range systemBeforeUpdateHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeDeleteHooks executes all "before Delete" hooks.
func (o *System) doBeforeDeleteHooks(exec boil.Executor) (err error) {
	for _, hook := range systemBeforeDeleteHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpsertHooks executes all "before Upsert" hooks.
func (o *System) doBeforeUpsertHooks(exec boil.Executor) (err error) {
	for _, hook := range systemBeforeUpsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterInsertHooks executes all "after Insert" hooks.
func (o *System) doAfterInsertHooks(exec boil.Executor) (err error) {
	for _, hook := range systemAfterInsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterSelectHooks executes all "after Select" hooks.
func (o *System) doAfterSelectHooks(exec boil.Executor) (err error) {
	for _, hook := range systemAfterSelectHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpdateHooks executes all "after Update" hooks.
func (o *System) doAfterUpdateHooks(exec boil.Executor) (err error) {
	for _, hook := range systemAfterUpdateHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterDeleteHooks executes all "after Delete" hooks.
func (o *System) doAfterDeleteHooks(exec boil.Executor) (err error) {
	for _, hook := range systemAfterDeleteHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpsertHooks executes all "after Upsert" hooks.
func (o *System) doAfterUpsertHooks(exec boil.Executor) (err error) {
	for _, hook := range systemAfterUpsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// AddSystemHook registers your hook function for all future operations.
func AddSystemHook(hookPoint boil.HookPoint, systemHook SystemHook) {
	switch hookPoint {
	case boil.BeforeInsertHook:
		systemBeforeInsertHooks = append(systemBeforeInsertHooks, systemHook)
	case boil.BeforeUpdateHook:
		systemBeforeUpdateHooks = append(systemBeforeUpdateHooks, systemHook)
	case boil.BeforeDeleteHook:
		systemBeforeDeleteHooks = append(systemBeforeDeleteHooks, systemHook)
	case boil.BeforeUpsertHook:
		systemBeforeUpsertHooks = append(systemBeforeUpsertHooks, systemHook)
	case boil.AfterInsertHook:
		systemAfterInsertHooks = append(systemAfterInsertHooks, systemHook)
	case boil.AfterSelectHook:
		systemAfterSelectHooks = append(systemAfterSelectHooks, systemHook)
	case boil.AfterUpdateHook:
		systemAfterUpdateHooks = append(systemAfterUpdateHooks, systemHook)
	case boil.AfterDeleteHook:
		systemAfterDeleteHooks = append(systemAfterDeleteHooks, systemHook)
	case boil.AfterUpsertHook:
		systemAfterUpsertHooks = append(systemAfterUpsertHooks, systemHook)
	}
}

// OneG returns a single system record from the query using the global executor.
func (q systemQuery) OneG() (*System, error) {
	return q.One(boil.GetDB())
}

// One returns a single system record from the query.
func (q systemQuery) One(exec boil.Executor) (*System, error) {
	o := &System{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(nil, exec, o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for system")
	}

	if err := o.doAfterSelectHooks(exec); err != nil {
		return o, err
	}

	return o, nil
}

// AllG returns all System records from the query using the global executor.
func (q systemQuery) AllG() (SystemSlice, error) {
	return q.All(boil.GetDB())
}

// All returns all System records from the query.
func (q systemQuery) All(exec boil.Executor) (SystemSlice, error) {
	var o []*System

	err := q.Bind(nil, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to System slice")
	}

	if len(systemAfterSelectHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterSelectHooks(exec); err != nil {
				return o, err
			}
		}
	}

	return o, nil
}

// CountG returns the count of all System records in the query, and panics on error.
func (q systemQuery) CountG() (int64, error) {
	return q.Count(boil.GetDB())
}

// Count returns the count of all System records in the query.
func (q systemQuery) Count(exec boil.Executor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow(exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count system rows")
	}

	return count, nil
}

// ExistsG checks if the row exists in the table, and panics on error.
func (q systemQuery) ExistsG() (bool, error) {
	return q.Exists(boil.GetDB())
}

// Exists checks if the row exists in the table.
func (q systemQuery) Exists(exec boil.Executor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow(exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if system exists")
	}

	return count > 0, nil
}

// Components retrieves all the component's Components with an executor.
func (o *System) Components(mods ...qm.QueryMod) componentQuery {
	var queryMods []qm.QueryMod
	if len(mods) != 0 {
		queryMods = append(queryMods, mods...)
	}

	queryMods = append(queryMods,
		qm.Where("\"component\".\"system_id\"=?", o.ID),
	)

	query := Components(queryMods...)
	queries.SetFrom(query.Query, "\"component\"")

	if len(queries.GetSelect(query.Query)) == 0 {
		queries.SetSelect(query.Query, []string{"\"component\".*"})
	}

	return query
}

// LoadComponents allows an eager lookup of values, cached into the
// loaded structs of the objects. This is for a 1-M or N-M relationship.
func (systemL) LoadComponents(e boil.Executor, singular bool, maybeSystem interface{}, mods queries.Applicator) error {
	var slice []*System
	var object *System

	if singular {
		object = maybeSystem.(*System)
	} else {
		slice = *maybeSystem.(*[]*System)
	}

	args := make([]interface{}, 0, 1)
	if singular {
		if object.R == nil {
			object.R = &systemR{}
		}
		args = append(args, object.ID)
	} else {
	Outer:
		for _, obj := range slice {
			if obj.R == nil {
				obj.R = &systemR{}
			}

			for _, a := range args {
				if queries.Equal(a, obj.ID) {
					continue Outer
				}
			}

			args = append(args, obj.ID)
		}
	}

	if len(args) == 0 {
		return nil
	}

	query := NewQuery(
		qm.From(`component`),
		qm.WhereIn(`component.system_id in ?`, args...),
	)
	if mods != nil {
		mods.Apply(query)
	}

	results, err := query.Query(e)
	if err != nil {
		return errors.Wrap(err, "failed to eager load component")
	}

	var resultSlice []*Component
	if err = queries.Bind(results, &resultSlice); err != nil {
		return errors.Wrap(err, "failed to bind eager loaded slice component")
	}

	if err = results.Close(); err != nil {
		return errors.Wrap(err, "failed to close results in eager load on component")
	}
	if err = results.Err(); err != nil {
		return errors.Wrap(err, "error occurred during iteration of eager loaded relations for component")
	}

	if len(componentAfterSelectHooks) != 0 {
		for _, obj := range resultSlice {
			if err := obj.doAfterSelectHooks(e); err != nil {
				return err
			}
		}
	}
	if singular {
		object.R.Components = resultSlice
		for _, foreign := range resultSlice {
			if foreign.R == nil {
				foreign.R = &componentR{}
			}
			foreign.R.System = object
		}
		return nil
	}

	for _, foreign := range resultSlice {
		for _, local := range slice {
			if queries.Equal(local.ID, foreign.SystemID) {
				local.R.Components = append(local.R.Components, foreign)
				if foreign.R == nil {
					foreign.R = &componentR{}
				}
				foreign.R.System = local
				break
			}
		}
	}

	return nil
}

// AddComponentsG adds the given related objects to the existing relationships
// of the system, optionally inserting them as new records.
// Appends related to o.R.Components.
// Sets related.R.System appropriately.
// Uses the global database handle.
func (o *System) AddComponentsG(insert bool, related ...*Component) error {
	return o.AddComponents(boil.GetDB(), insert, related...)
}

// AddComponents adds the given related objects to the existing relationships
// of the system, optionally inserting them as new records.
// Appends related to o.R.Components.
// Sets related.R.System appropriately.
func (o *System) AddComponents(exec boil.Executor, insert bool, related ...*Component) error {
	var err error
	for _, rel := range related {
		if insert {
			queries.Assign(&rel.SystemID, o.ID)
			if err = rel.Insert(exec, boil.Infer()); err != nil {
				return errors.Wrap(err, "failed to insert into foreign table")
			}
		} else {
			updateQuery := fmt.Sprintf(
				"UPDATE \"component\" SET %s WHERE %s",
				strmangle.SetParamNames("\"", "\"", 1, []string{"system_id"}),
				strmangle.WhereClause("\"", "\"", 2, componentPrimaryKeyColumns),
			)
			values := []interface{}{o.ID, rel.ID}

			if boil.DebugMode {
				fmt.Fprintln(boil.DebugWriter, updateQuery)
				fmt.Fprintln(boil.DebugWriter, values)
			}
			if _, err = exec.Exec(updateQuery, values...); err != nil {
				return errors.Wrap(err, "failed to update foreign table")
			}

			queries.Assign(&rel.SystemID, o.ID)
		}
	}

	if o.R == nil {
		o.R = &systemR{
			Components: related,
		}
	} else {
		o.R.Components = append(o.R.Components, related...)
	}

	for _, rel := range related {
		if rel.R == nil {
			rel.R = &componentR{
				System: o,
			}
		} else {
			rel.R.System = o
		}
	}
	return nil
}

// SetComponentsG removes all previously related items of the
// system replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.System's Components accordingly.
// Replaces o.R.Components with related.
// Sets related.R.System's Components accordingly.
// Uses the global database handle.
func (o *System) SetComponentsG(insert bool, related ...*Component) error {
	return o.SetComponents(boil.GetDB(), insert, related...)
}

// SetComponents removes all previously related items of the
// system replacing them completely with the passed
// in related items, optionally inserting them as new records.
// Sets o.R.System's Components accordingly.
// Replaces o.R.Components with related.
// Sets related.R.System's Components accordingly.
func (o *System) SetComponents(exec boil.Executor, insert bool, related ...*Component) error {
	query := "update \"component\" set \"system_id\" = null where \"system_id\" = $1"
	values := []interface{}{o.ID}
	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, query)
		fmt.Fprintln(boil.DebugWriter, values)
	}
	_, err := exec.Exec(query, values...)
	if err != nil {
		return errors.Wrap(err, "failed to remove relationships before set")
	}

	if o.R != nil {
		for _, rel := range o.R.Components {
			queries.SetScanner(&rel.SystemID, nil)
			if rel.R == nil {
				continue
			}

			rel.R.System = nil
		}

		o.R.Components = nil
	}
	return o.AddComponents(exec, insert, related...)
}

// RemoveComponentsG relationships from objects passed in.
// Removes related items from R.Components (uses pointer comparison, removal does not keep order)
// Sets related.R.System.
// Uses the global database handle.
func (o *System) RemoveComponentsG(related ...*Component) error {
	return o.RemoveComponents(boil.GetDB(), related...)
}

// RemoveComponents relationships from objects passed in.
// Removes related items from R.Components (uses pointer comparison, removal does not keep order)
// Sets related.R.System.
func (o *System) RemoveComponents(exec boil.Executor, related ...*Component) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	for _, rel := range related {
		queries.SetScanner(&rel.SystemID, nil)
		if rel.R != nil {
			rel.R.System = nil
		}
		if _, err = rel.Update(exec, boil.Whitelist("system_id")); err != nil {
			return err
		}
	}
	if o.R == nil {
		return nil
	}

	for _, rel := range related {
		for i, ri := range o.R.Components {
			if rel != ri {
				continue
			}

			ln := len(o.R.Components)
			if ln > 1 && i < ln-1 {
				o.R.Components[i] = o.R.Components[ln-1]
			}
			o.R.Components = o.R.Components[:ln-1]
			break
		}
	}

	return nil
}

// Systems retrieves all the records using an executor.
func Systems(mods ...qm.QueryMod) systemQuery {
	mods = append(mods, qm.From("\"system\""))
	return systemQuery{NewQuery(mods...)}
}

// FindSystemG retrieves a single record by ID.
func FindSystemG(iD string, selectCols ...string) (*System, error) {
	return FindSystem(boil.GetDB(), iD, selectCols...)
}

// FindSystem retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindSystem(exec boil.Executor, iD string, selectCols ...string) (*System, error) {
	systemObj := &System{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"system\" where \"id\"=$1", sel,
	)

	q := queries.Raw(query, iD)

	err := q.Bind(nil, exec, systemObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from system")
	}

	if err = systemObj.doAfterSelectHooks(exec); err != nil {
		return systemObj, err
	}

	return systemObj, nil
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *System) InsertG(columns boil.Columns) error {
	return o.Insert(boil.GetDB(), columns)
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *System) Insert(exec boil.Executor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no system provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = currTime
	}

	if err := o.doBeforeInsertHooks(exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(systemColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	systemInsertCacheMut.RLock()
	cache, cached := systemInsertCache[key]
	systemInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			systemAllColumns,
			systemColumnsWithDefault,
			systemColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(systemType, systemMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(systemType, systemMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"system\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"system\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into system")
	}

	if !cached {
		systemInsertCacheMut.Lock()
		systemInsertCache[key] = cache
		systemInsertCacheMut.Unlock()
	}

	return o.doAfterInsertHooks(exec)
}

// UpdateG a single System record using the global executor.
// See Update for more documentation.
func (o *System) UpdateG(columns boil.Columns) (int64, error) {
	return o.Update(boil.GetDB(), columns)
}

// Update uses an executor to update the System.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *System) Update(exec boil.Executor, columns boil.Columns) (int64, error) {
	currTime := time.Now().In(boil.GetLocation())

	o.UpdatedAt = currTime

	var err error
	if err = o.doBeforeUpdateHooks(exec); err != nil {
		return 0, err
	}
	key := makeCacheKey(columns, nil)
	systemUpdateCacheMut.RLock()
	cache, cached := systemUpdateCache[key]
	systemUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			systemAllColumns,
			systemPrimaryKeyColumns,
		)

		if !columns.IsWhitelist() {
			wl = strmangle.SetComplement(wl, []string{"created_at"})
		}
		if len(wl) == 0 {
			return 0, errors.New("models: unable to update system, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"system\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, systemPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(systemType, systemMapping, append(wl, systemPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, values)
	}
	var result sql.Result
	result, err = exec.Exec(cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update system row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for system")
	}

	if !cached {
		systemUpdateCacheMut.Lock()
		systemUpdateCache[key] = cache
		systemUpdateCacheMut.Unlock()
	}

	return rowsAff, o.doAfterUpdateHooks(exec)
}

// UpdateAllG updates all rows with the specified column values.
func (q systemQuery) UpdateAllG(cols M) (int64, error) {
	return q.UpdateAll(boil.GetDB(), cols)
}

// UpdateAll updates all rows with the specified column values.
func (q systemQuery) UpdateAll(exec boil.Executor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.Exec(exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for system")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for system")
	}

	return rowsAff, nil
}

// UpdateAllG updates all rows with the specified column values.
func (o SystemSlice) UpdateAllG(cols M) (int64, error) {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o SystemSlice) UpdateAll(exec boil.Executor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("models: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), systemPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"system\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, systemPrimaryKeyColumns, len(o)))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}
	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in system slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all system")
	}
	return rowsAff, nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *System) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, insertColumns)
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *System) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no system provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}
	o.UpdatedAt = currTime

	if err := o.doBeforeUpsertHooks(exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(systemColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	systemUpsertCacheMut.RLock()
	cache, cached := systemUpsertCache[key]
	systemUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := insertColumns.InsertColumnSet(
			systemAllColumns,
			systemColumnsWithDefault,
			systemColumnsWithoutDefault,
			nzDefaults,
		)
		update := updateColumns.UpdateColumnSet(
			systemAllColumns,
			systemPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert system, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(systemPrimaryKeyColumns))
			copy(conflict, systemPrimaryKeyColumns)
		}
		cache.query = buildUpsertQueryPostgres(dialect, "\"system\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(systemType, systemMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(systemType, systemMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}
	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(returns...)
		if err == sql.ErrNoRows {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert system")
	}

	if !cached {
		systemUpsertCacheMut.Lock()
		systemUpsertCache[key] = cache
		systemUpsertCacheMut.Unlock()
	}

	return o.doAfterUpsertHooks(exec)
}

// DeleteG deletes a single System record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *System) DeleteG() (int64, error) {
	return o.Delete(boil.GetDB())
}

// Delete deletes a single System record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *System) Delete(exec boil.Executor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no System provided for delete")
	}

	if err := o.doBeforeDeleteHooks(exec); err != nil {
		return 0, err
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), systemPrimaryKeyMapping)
	sql := "DELETE FROM \"system\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}
	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from system")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for system")
	}

	if err := o.doAfterDeleteHooks(exec); err != nil {
		return 0, err
	}

	return rowsAff, nil
}

func (q systemQuery) DeleteAllG() (int64, error) {
	return q.DeleteAll(boil.GetDB())
}

// DeleteAll deletes all matching rows.
func (q systemQuery) DeleteAll(exec boil.Executor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no systemQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.Exec(exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from system")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for system")
	}

	return rowsAff, nil
}

// DeleteAllG deletes all rows in the slice.
func (o SystemSlice) DeleteAllG() (int64, error) {
	return o.DeleteAll(boil.GetDB())
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o SystemSlice) DeleteAll(exec boil.Executor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	if len(systemBeforeDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doBeforeDeleteHooks(exec); err != nil {
				return 0, err
			}
		}
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), systemPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"system\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, systemPrimaryKeyColumns, len(o))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}
	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from system slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for system")
	}

	if len(systemAfterDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterDeleteHooks(exec); err != nil {
				return 0, err
			}
		}
	}

	return rowsAff, nil
}

// ReloadG refetches the object from the database using the primary keys.
func (o *System) ReloadG() error {
	if o == nil {
		return errors.New("models: no System provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *System) Reload(exec boil.Executor) error {
	ret, err := FindSystem(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *SystemSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("models: empty SystemSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *SystemSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := SystemSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), systemPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"system\".* FROM \"system\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, systemPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(nil, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in SystemSlice")
	}

	*o = slice

	return nil
}

// SystemExistsG checks if the System row exists.
func SystemExistsG(iD string) (bool, error) {
	return SystemExists(boil.GetDB(), iD)
}

// SystemExists checks if the System row exists.
func SystemExists(exec boil.Executor, iD string) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"system\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, iD)
	}
	row := exec.QueryRow(sql, iD)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if system exists")
	}

	return exists, nil
}

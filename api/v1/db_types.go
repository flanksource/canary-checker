package v1

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type ResourceSelectors []ResourceSelector

type CanarySelectors []CanarySelector

func (rs ResourceSelectors) Value() (driver.Value, error) {
	if len(rs) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(rs)
}

func (rs *ResourceSelectors) Scan(val interface{}) error {
	if val == nil {
		*rs = ResourceSelectors{}
		return nil
	}
	var ba []byte
	switch v := val.(type) {
	case []byte:
		ba = v
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal ResourceSelectors value:", val))
	}
	return json.Unmarshal(ba, rs)
}

// GormDataType gorm common data type
func (rs ResourceSelectors) GormDataType() string {
	return "resourceSelectors"
}

// GormDBDataType gorm db data type
func (ResourceSelectors) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "JSON"
	case "postgres":
		return "JSONB"
	case "sqlserver":
		return "NVARCHAR(MAX)"
	}
	return ""
}

func (rs ResourceSelectors) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(rs)
	return gorm.Expr("?", string(data))
}

func (cs CanarySelectors) Value() (driver.Value, error) {
	if len(cs) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(cs)
}

func (cs *CanarySelectors) Scan(val interface{}) error {
	if val == nil {
		*cs = CanarySelectors{}
		return nil
	}
	var ba []byte
	switch v := val.(type) {
	case []byte:
		ba = v
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal CanarySelectors value:", val))
	}
	return json.Unmarshal(ba, cs)
}

// GormDataType gorm common data type
func (cs CanarySelectors) GormDataType() string {
	return "canarySelectors"
}

// GormDBDataType gorm db data type
func (CanarySelectors) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "JSON"
	case "postgres":
		return "JSONB"
	case "sqlserver":
		return "NVARCHAR(MAX)"
	}
	return ""
}

func (cs CanarySelectors) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(cs)
	return gorm.Expr("?", string(data))
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (s Summary) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (s *Summary) Scan(val interface{}) error {
	if val == nil {
		*s = Summary{}
		return nil
	}
	var ba []byte
	switch v := val.(type) {
	case []byte:
		ba = v
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal properties value:", val))
	}
	err := json.Unmarshal(ba, s)
	return err
}

// GormDataType gorm common data type
func (Summary) GormDataType() string {
	return "summary"
}

func (Summary) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "TEXT"
	case "postgres":
		return "JSONB"
	case "sqlserver":
		return "NVARCHAR(MAX)"
	}
	return ""
}

func (s Summary) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(s)
	return gorm.Expr("?", data)
}

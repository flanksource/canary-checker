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

const (
	SQLServerType = "sqlserver"
	PostgresType  = "postgres"
	SqliteType    = "sqlite"
	jsonType      = "json"
	jsonbType     = "JSONB"
	nvarcharType  = "NVARCHAR(MAX)"
)

type ComponentChecks []ComponentCheck

func (cs ComponentChecks) Value() (driver.Value, error) {
	if len(cs) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(cs)
}

func (cs *ComponentChecks) Scan(val interface{}) error {
	if val == nil {
		*cs = ComponentChecks{}
		return nil
	}
	var ba []byte
	switch v := val.(type) {
	case []byte:
		ba = v
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal componentChecks value:", val))
	}
	return json.Unmarshal(ba, cs)
}

// GormDataType gorm common data type
func (cs ComponentChecks) GormDataType() string {
	return "componentChecks"
}

// GormDBDataType gorm db data type
func (ComponentChecks) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case SqliteType:
		return jsonType
	case PostgresType:
		return jsonbType
	case SQLServerType:
		return nvarcharType
	}
	return ""
}

func (cs ComponentChecks) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(cs)
	return gorm.Expr("?", string(data))
}

package db

import (
	"encoding/json"

	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func mapToJson(m map[string]string) null.JSON {
	b, err := json.Marshal(m)
	if err != nil {
		return null.JSON{}
	}
	return null.JSONFrom(b)
}

func getColumnsFromString(cols ...string) boil.Columns {
	if len(cols) == 0 {
		return boil.Infer()
	}
	return boil.Whitelist(cols...)
}

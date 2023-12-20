package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/commons/duration"
)

func ConvertNamedParamsDebug(sql string, namedArgs map[string]interface{}) string {
	// Loop the named args and replace with placeholders
	for pname, pval := range namedArgs {
		sql = strings.ReplaceAll(sql, ":"+pname, fmt.Sprintf("%v", pval))
	}
	return sql
}

func ConvertNamedParams(sql string, namedArgs map[string]interface{}) (string, []interface{}) {
	i := 1
	var args []interface{}
	// Loop the named args and replace with placeholders
	for pname, pval := range namedArgs {
		sql = strings.ReplaceAll(sql, ":"+pname, fmt.Sprint(`$`, i))
		args = append(args, pval)
		i++
	}
	return sql, args
}

func timeV(v interface{}) (*time.Time, error) {
	if v == nil {
		return nil, nil
	}
	switch v := v.(type) {
	case time.Time:
		return &v, nil
	case time.Duration:
		t := time.Now().Add(v * -1)
		return &t, nil
	case string:
		if v == "" {
			return nil, nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return &t, nil
		} else if d, err := duration.ParseDuration(v); err == nil {
			t := time.Now().Add(time.Duration(d) * -1)
			return &t, nil
		}
		return nil, fmt.Errorf("time must be a duration or RFC3339 timestamp")
	}
	return nil, fmt.Errorf("unknown time type %T", v)
}

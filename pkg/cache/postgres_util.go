package cache

import (
	"fmt"
	"strings"
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

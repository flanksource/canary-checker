package http

import (
	"fmt"

	"github.com/PaesslerAG/jsonpath"
	"github.com/pkg/errors"
)

type JSON struct {
	Value interface{}
}

func (j JSON) JSONPath(path string) (string, error) {
	jsonResult, err := jsonpath.Get(path, j.Value)
	if err != nil {
		return "", errors.Wrapf(err, "could not extract %v", path)
	}
	return fmt.Sprintf("%s", jsonResult), nil
}

package pkg

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/mitchellh/reflectwalk"
	"gopkg.in/flanksource/yaml.v3"
)

// ParseConfig : Read config file
func ParseConfig(configfile string) v1.CanarySpec {
	config := v1.CanarySpec{}
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		logger.Infof("yamlFile.Get err   #%v ", err)
	}
	yamlerr := yaml.Unmarshal([]byte(data), &config)
	if yamlerr != nil {
		logger.Fatalf("error: %v", yamlerr)
	}
	return ApplyTemplates(config)
}

type StructTemplater struct {
	Values map[string]string
}

// this func is required to fulfil the reflectwalk.StructWalker interface
func (w StructTemplater) Struct(reflect.Value) error {
	return nil
}

func (w StructTemplater) StructField(f reflect.StructField, v reflect.Value) error {
	if v.CanSet() && v.Kind() == reflect.String {
		v.SetString(w.Template(v.String()))
	}
	return nil
}

func (w StructTemplater) Template(val string) string {
	if strings.HasPrefix(val, "$") {
		key := strings.TrimRight(strings.TrimLeft(val[1:], "("), ")")
		env := w.Values[key]
		if env != "" {
			return env
		}
	}
	return val
}

func ApplyTemplates(config v1.CanarySpec) v1.CanarySpec {
	var values = make(map[string]string)
	for _, environ := range os.Environ() {
		values[strings.Split(environ, "=")[0]] = strings.Split(environ, "=")[1]
	}
	if err := reflectwalk.Walk(&config, StructTemplater{Values: values}); err != nil {
		fmt.Println(err)
	}
	return config
}

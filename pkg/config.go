package pkg

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons/ktemplate"
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
	envVar := GetEnvMap()
	parsed, err := ApplyGomplate(string(data), envVar)
	if err != nil {
		logger.Infof("Gomplate parsing err   #%v ", err)
	}
	yamlerr := yaml.Unmarshal([]byte(parsed), &config)
	if yamlerr != nil {
		logger.Fatalf("error: %v", yamlerr)
	}
	return ApplyTemplates(config, envVar)
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

func GetEnvMap() map[string]string {
	var values = make(map[string]string)
	for _, environ := range os.Environ() {
		values[strings.Split(environ, "=")[0]] = strings.Split(environ, "=")[1]
	}
	return values
}

func ApplyTemplates(config v1.CanarySpec, values map[string]string) v1.CanarySpec {
	if err := reflectwalk.Walk(&config, StructTemplater{Values: values}); err != nil {
		fmt.Println(err)
	}
	return config
}

func ApplyGomplate(rawInput string, values map[string]string) (string, error) {
	k8sClient, err := NewK8sClient()
	if err != nil {
		return rawInput, errors.Wrap(err, "Failed to generate new k8s client")
	}
	fns := ktemplate.NewFunctions(k8sClient)
	return fns.Template(rawInput, values)
}

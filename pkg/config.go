package pkg

import (
	"fmt"
	"go.opencensus.io/resource/resourcekeys"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"github.com/mitchellh/reflectwalk"
	"gopkg.in/flanksource/yaml.v3"
	"github.com/flanksource/kommons/ktemplate"
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

func ApplyTemplates(config v1.CanarySpec) v1.CanarySpec {
	var values = make(map[string]string)
	for _, environ := range os.Environ() {
		values[strings.Split(environ, "=")[0]] = strings.Split(environ, "=")[1]
	}
	k8sclient, err := NewK8sClient()
	if err != nil {
		logger.Warnf("Could not create k8s client for templating: %v", err)
	}
	if err := reflectwalk.Walk(&config, ktemplate.StructTemplater{
		Values:    values,
		Clientset: k8sclient,
	}); err != nil {
		fmt.Println(err)
	}
	return config
}

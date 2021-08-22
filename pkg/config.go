package pkg

import (
	"io/ioutil"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
	"gopkg.in/flanksource/yaml.v3"
)

// ParseConfig : Read config file
func ParseConfig(configfile string) v1.CanarySpec {
	config := v1.CanarySpec{}
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		logger.Infof("yamlFile.Get err   #%v ", err)
	}
	yamlerr := yaml.Unmarshal(data, &config)
	if yamlerr != nil {
		logger.Fatalf("error: %v", yamlerr)
	}
	return config
}

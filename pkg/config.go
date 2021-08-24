package pkg

import (
	"io/ioutil"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"gopkg.in/flanksource/yaml.v3"
)

// ParseConfig : Read config file
func ParseConfig(configfile string) (*v1.CanarySpec, error) {
	config := v1.CanarySpec{}
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		return nil, err
	}
	yamlerr := yaml.Unmarshal(data, &config)
	if yamlerr != nil {
		return nil, yamlerr
	}
	return &config, nil
}

package pkg

import (
	"bytes"
	"io/ioutil"
	"os"

	v1 "github.com/flanksource/canary-checker/api/v1"

	"gopkg.in/flanksource/yaml.v3"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

// ParseConfig : Read config file
func ParseConfig(configfile string) (*v1.Canary, error) {
	config := v1.Canary{}
	var data []byte
	var err error
	if configfile == "-" {
		if data, err = ioutil.ReadAll(os.Stdin); err != nil {
			return nil, err
		}
	} else {
		if data, err = ioutil.ReadFile(configfile); err != nil {
			return nil, err
		}
	}

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)

	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	if len(config.Spec.GetAllChecks()) == 0 {
		// try just the specs:
		spec := v1.CanarySpec{}

		if yamlerr := yaml.Unmarshal(data, &spec); yamlerr != nil {
			return nil, yamlerr
		}
		config.Spec = spec
	}

	return &config, nil
}

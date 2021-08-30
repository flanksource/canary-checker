package pkg

import (
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"

	"gopkg.in/flanksource/yaml.v3"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

// ParseConfig : Read config file
func ParseConfig(configfile string) ([]v1.Canary, error) {
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

	var canaries []v1.Canary
	re := regexp.MustCompile(`(?m)^---\n`)
	for _, chunk := range re.Split(string(data), -1) {
		config := v1.Canary{}
		decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(chunk), 1024)

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
		canaries = append(canaries, config)
	}

	return canaries, nil
}

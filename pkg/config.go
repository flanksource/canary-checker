package pkg

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/gomplate/v3"

	"gopkg.in/flanksource/yaml.v3"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func readFile(path string) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		if data, err = io.ReadAll(os.Stdin); err != nil {
			return "", err
		}
	} else {
		if data, err = os.ReadFile(path); err != nil {
			return "", err
		}
	}
	return string(data), nil
}

func parseDataFile(file string) (map[string]any, error) {
	var d map[string]any
	data, err := readFile(file)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal([]byte(data), &d)
	return d, err
}

func template(content string, data map[string]any) (string, error) {
	return gomplate.RunTemplate(data, gomplate.Template{
		Template: content,
	})
}

func ParseTopology(configFile, datafile string) ([]*Topology, error) {
	configs, err := readFile(configFile)
	if err != nil {
		return nil, err
	}

	if datafile != "" {
		data, err := parseDataFile(datafile)
		if err != nil {
			return nil, err
		}
		configs, err = template(configs, data)
		if err != nil {
			return nil, err
		}
	}

	var systems []*Topology
	re := regexp.MustCompile(`(?m)^---\n`)
	for _, chunk := range re.Split(configs, -1) {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		config := v1.Topology{}
		decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(chunk), 1024)

		if err := decoder.Decode(&config); err != nil {
			return nil, err
		}

		if config.IsEmpty() {
			// try just the specs:
			spec := v1.TopologySpec{}

			if yamlerr := yaml.Unmarshal([]byte(chunk), &spec); yamlerr != nil {
				return nil, yamlerr
			}
			config.Spec = spec
		}
		if config.Name == "" {
			config.Name = CleanupFilename(configFile)
		}
		v1Topology := TopologyFromV1(&config)
		systems = append(systems, &v1Topology)
	}

	return systems, nil
}

// ParseConfig : Read config file
func ParseConfig(configfile string, datafile string) ([]v1.Canary, error) {
	configs, err := readFile(configfile)
	if err != nil {
		return nil, err
	}

	if datafile != "" {
		data, err := parseDataFile(datafile)
		if err != nil {
			return nil, err
		}
		configs, err = template(configs, data)
		if err != nil {
			return nil, err
		}
	}

	var canaries []v1.Canary
	re := regexp.MustCompile(`(?m)^---\n`)
	for _, chunk := range re.Split(configs, -1) {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		config := v1.Canary{}
		decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(chunk), 1024)

		if err := decoder.Decode(&config); err != nil {
			return nil, err
		}

		if len(config.Spec.GetAllChecks()) == 0 && config.Spec.Webhook == nil {
			// try just the specs:
			spec := v1.CanarySpec{}

			if yamlerr := yaml.Unmarshal([]byte(chunk), &spec); yamlerr != nil {
				return nil, yamlerr
			}
			config.Spec = spec
		}
		canaries = append(canaries, config)
	}

	return canaries, nil
}

func CleanupFilename(fileName string) string {
	removeSuffix := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	return strings.ReplaceAll(removeSuffix, "_", "")
}

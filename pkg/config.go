package pkg

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	gotemplate "text/template"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/text"

	"gopkg.in/flanksource/yaml.v3"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func readFile(path string) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		if data, err = ioutil.ReadAll(os.Stdin); err != nil {
			return "", err
		}
	} else {
		if data, err = ioutil.ReadFile(path); err != nil {
			return "", err
		}
	}
	return string(data), nil
}

func parseDataFile(file string) (interface{}, error) {
	var d interface{}
	data, err := readFile(file)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal([]byte(data), &d)
	return d, err
}

func template(content string, data interface{}) (string, error) {
	tpl := gotemplate.New("")
	tpl, err := tpl.Funcs(text.GetTemplateFuncs()).Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error executing template %s: %v", strings.Split(content, "\n")[0], err)
	}
	fmt.Println(buf.String())
	return strings.TrimSpace(buf.String()), nil
}

func ParseSystems(configFile, datafile string) ([]v1.SystemTemplate, error) {
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

	var systems []v1.SystemTemplate
	re := regexp.MustCompile(`(?m)^---\n`)
	for _, chunk := range re.Split(configs, -1) {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		config := v1.SystemTemplate{}
		decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(chunk), 1024)

		if err := decoder.Decode(&config); err != nil {
			return nil, err
		}

		if config.IsEmpty() {
			// try just the specs:
			spec := v1.SystemTemplateSpec{}

			if yamlerr := yaml.Unmarshal([]byte(chunk), &spec); yamlerr != nil {
				return nil, yamlerr
			}
			config.Spec = spec
		}
		if config.Name == "" {
			config.Name = CleanupFilename(configFile)
		}
		systems = append(systems, config)
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

		if len(config.Spec.GetAllChecks()) == 0 {
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
	return strings.Replace(removeSuffix, "_", "", -1)
}

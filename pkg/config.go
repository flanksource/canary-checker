package pkg

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

// ParseConfig : Read config file
func ParseConfig(configfile string) Config {
	config := Config{}
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	yamlerr := yaml.Unmarshal([]byte(data), &config)
	if yamlerr != nil {
		log.Fatalf("error: %v", yamlerr)
	}
	return config
}




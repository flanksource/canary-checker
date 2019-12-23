package pkg

import (
	"fmt"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

// ReadConfig : Read config and call CheckConfig
func ReadConfig(configfile string) Config {
	config := Config{}
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	yamlerr := yaml.Unmarshal([]byte(data), &config)
	if yamlerr != nil {
		log.Fatalf("error: %v", yamlerr)
	}
	for _, conf := range config.HTTP {
		m := map[string]interface{}{
			"http": conf,
		}
		for _, i := range CheckConfig(m) {
			fmt.Println(i)
		}
	}
	return config
}

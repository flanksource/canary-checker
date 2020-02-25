package pkg

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

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
	return ApplyTemplates(config)
}

func ApplyTemplates(config Config) Config {

	buckets := []S3{}
	for _, s3 := range config.S3 {
		s3.AccessKey = template(s3.AccessKey)
		s3.SecretKey = template(s3.SecretKey)
		buckets = append(buckets, s3)
	}
	config.S3 = buckets
	return config
}

func template(val string) string {
	if strings.HasPrefix(val, "$") {
		env := os.Getenv(val[1:])
		if env != "" {
			return env
		}
	}
	return val
}

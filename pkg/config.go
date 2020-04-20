package pkg

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/flanksource/yaml"
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
	s3Buckets := []S3Bucket{}
	for _, bucket := range config.S3Bucket {
		bucket.AccessKey = template(bucket.AccessKey)
		bucket.SecretKey = template(bucket.SecretKey)
		s3Buckets = append(s3Buckets, bucket)
	}
	config.S3Bucket = s3Buckets
	ldap := []LDAP{}
	for _, item := range config.LDAP {
		item.Password = template(item.Password)
		item.Username = template(item.Username)
		ldap = append(ldap, item)
	}
	config.LDAP = ldap
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

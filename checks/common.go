package checks

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/kommons"
)

func GetAuthValues(auth *v1.Authentication, client *kommons.Client, namespace string) (v1.Authentication, error) {
	authentication := &v1.Authentication{
		Username: kommons.EnvVar{
			Value: "",
		},
		Password: kommons.EnvVar{
			Value: "",
		},
	}
	// in case nil we are sending empty string values for username and password
	if auth == nil {
		return *authentication, nil
	}
	_, username, err := client.GetEnvValue(auth.Username, namespace)
	if err != nil {
		return *authentication, err
	}
	authentication.Username = kommons.EnvVar{
		Value: username,
	}
	_, password, err := client.GetEnvValue(auth.Password, namespace)
	if err != nil {
		return *authentication, err
	}
	authentication.Password = kommons.EnvVar{
		Value: password,
	}
	return *authentication, err
}

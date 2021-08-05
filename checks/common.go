package checks

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/kommons"
)

func GetAuthValues(auth *v1.Authentication, client *kommons.Client, namespace string) (username, password string, err error) {
	if auth == nil {
		return
	}
	_, username, err = client.GetEnvValue(auth.Username, namespace)
	if err != nil {
		return
	}
	_, password, err = client.GetEnvValue(auth.Password, namespace)
	if err != nil {
		return
	}
	return
}

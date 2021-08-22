package checks

import (
	"os"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/kommons"
)

func GetAuthValues(auth *v1.Authentication, client *kommons.Client, namespace string) (*v1.Authentication, error) {
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
		return nil, nil
	}
	_, username, err := client.GetEnvValue(auth.Username, namespace)
	if err != nil {
		return nil, err
	}
	authentication.Username = kommons.EnvVar{
		Value: username,
	}
	_, password, err := client.GetEnvValue(auth.Password, namespace)
	if err != nil {
		return nil, err
	}
	authentication.Password = kommons.EnvVar{
		Value: password,
	}
	return authentication, err
}

type FolderCheck struct {
	Oldest  Duration
	Newest  Duration
	MinSize Size
	MaxSize Size
	Files   []os.FileInfo
}

func (f FolderCheck) Test(test v1.FolderTest) string {
	if test.MinAge != nil && f.Newest.Duration < test.MinAge.Duration {

	}

	if test.MaxAge != nil && f.Oldest.Duration > test.MaxAge.Duration {

	}

	if test.MinCount != nil && len(f.Files) < *test.MinCount {

	}
	if test.MaxCount != nil && len(f.Files) > *test.MaxCount {

	}
	return ""
}

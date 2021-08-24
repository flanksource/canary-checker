package checks

import (
	"fmt"
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
		return authentication, nil
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
	MinSize int64
	MaxSize int64
	Files   []os.FileInfo
}

func (f FolderCheck) Test(test v1.FolderTest) string {
	if test.MinAge != nil && f.Newest.Duration < *test.GetMinAge() {
		return fmt.Sprintf("newest file is too old: %s < %s", f.Newest, *test.GetMinAge())
	}
	if test.MaxAge != nil && f.Oldest.Duration > *test.GetMaxAge() {
		return fmt.Sprintf("oldest file is too old: %s > %s", f.Oldest, *test.GetMaxAge())
	}
	if test.MinCount != nil && len(f.Files) < *test.MinCount {
		return fmt.Sprintf("too few files %d < %d", len(f.Files), *test.MinCount)
	}
	if test.MaxCount != nil && len(f.Files) > *test.MaxCount {
		return fmt.Sprintf("too many files %d > %d", len(f.Files), *test.MaxCount)
	}
	if test.MinSize != nil && f.MinSize < *test.MinSize {
		return fmt.Sprintf("min size is too small: %d < %d", f.MinSize, test.MinSize)
	}
	if test.MaxSize != nil && f.MaxSize < *test.MaxSize {
		return fmt.Sprintf("max size is too large: %d > %d", f.MaxSize, test.MaxSize)
	}
	return ""
}

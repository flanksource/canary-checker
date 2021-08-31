package checks

import (
	"fmt"
	"os"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/kommons"
	"github.com/robfig/cron/v3"
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
	Oldest  os.FileInfo
	Newest  os.FileInfo
	MinSize os.FileInfo
	MaxSize os.FileInfo
	Files   []os.FileInfo
}

func (result *FolderCheck) Append(file os.FileInfo) {
	if result.Oldest == nil || result.Oldest.ModTime().After(file.ModTime()) {
		result.Oldest = file
	}
	if result.Newest == nil || result.Newest.ModTime().Before(file.ModTime()) {
		result.Newest = file
	}
	if result.MinSize == nil || result.MinSize.Size() > file.Size() {
		result.MinSize = file
	}
	if result.MaxSize == nil || result.MaxSize.Size() < file.Size() {
		result.MaxSize = file
	}
	result.Files = append(result.Files, file)
}

func age(t time.Time) string {
	return utils.Age(time.Since(t))
}

func (f FolderCheck) Test(test v1.FolderTest) string {
	minAge, err := test.GetMinAge()
	if err != nil {
		return fmt.Sprintf("invalid duration %s: %v", test.MinAge, err)
	}
	maxAge, err := test.GetMaxAge()

	if test.MinCount != nil && len(f.Files) < *test.MinCount {
		return fmt.Sprintf("too few files %d < %d", len(f.Files), *test.MinCount)
	}
	if test.MaxCount != nil && len(f.Files) > *test.MaxCount {
		return fmt.Sprintf("too many files %d > %d", len(f.Files), *test.MaxCount)
	}

	if len(f.Files) == 0 {
		// nothing run age/size checks on
		return ""
	}

	if err != nil {
		return fmt.Sprintf("invalid duration %s: %v", test.MaxAge, err)
	}
	if minAge != nil && time.Since(f.Newest.ModTime()) < *minAge {
		return fmt.Sprintf("%s is too new: %s < %s", f.Newest.Name(), age(f.Newest.ModTime()), test.MinAge)
	}
	if maxAge != nil && time.Since(f.Oldest.ModTime()) > *maxAge {
		return fmt.Sprintf("%s is too old %s > %s", f.Oldest.Name(), age(f.Oldest.ModTime()), test.MaxAge)
	}

	if test.MinSize != "" {
		size, err := test.MinSize.Value()
		if err != nil {
			return fmt.Sprintf("%s is an invalid size: %s", test.MinSize, err)
		}
		if f.MinSize.Size() < *size {
			return fmt.Sprintf("%s is too small: %v < %v", f.MinSize.Name(), mb(f.MinSize.Size()), test.MinSize)
		}
	}

	if test.MaxSize != "" {
		size, err := test.MaxSize.Value()
		if err != nil {
			return fmt.Sprintf("%s is an invalid size: %s", test.MinSize, err)

		}
		if f.MaxSize.Size() < *size {
			return fmt.Sprintf("%s is too large: %v > %v", f.MaxSize.Name(), mb(f.MaxSize.Size()), test.MaxSize)
		}
	}
	return ""
}

func getNextRuntime(canary v1.Canary, lastRuntime time.Time) (*time.Time, error) {
	if canary.Spec.Schedule != "" {
		schedule, err := cron.ParseStandard(canary.Spec.Schedule)
		if err != nil {
			return nil, err
		}
		t := schedule.Next(time.Now())
		return &t, nil
	}
	t := lastRuntime.Add(time.Duration(canary.Spec.Interval) + time.Second)
	return &t, nil
}

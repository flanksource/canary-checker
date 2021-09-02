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

func (f *FolderCheck) Append(file os.FileInfo) {
	if f.Oldest == nil || f.Oldest.ModTime().After(file.ModTime()) {
		f.Oldest = file
	}
	if f.Newest == nil || f.Newest.ModTime().Before(file.ModTime()) {
		f.Newest = file
	}
	if f.MinSize == nil || f.MinSize.Size() > file.Size() {
		f.MinSize = file
	}
	if f.MaxSize == nil || f.MaxSize.Size() < file.Size() {
		f.MaxSize = file
	}
	f.Files = append(f.Files, file)
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

func GetDeadline(canary v1.Canary) time.Time {
	if canary.Spec.Schedule != "" {
		schedule, err := cron.ParseStandard(canary.Spec.Schedule)
		if err != nil {
			// cron syntax errors are handled elsewhere, default to a 10 second timeout
			return time.Now().Add(10 * time.Second)
		}
		return schedule.Next(time.Now())
	}
	return time.Now().Add(time.Duration(canary.Spec.Interval) * time.Second)
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
	t := lastRuntime.Add(time.Duration(canary.Spec.Interval) * time.Second)
	return &t, nil
}

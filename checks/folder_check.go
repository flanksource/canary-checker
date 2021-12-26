package checks

import (
	"fmt"
	"os"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
)

type Filesystem interface {
	Close()
	ReadDir(name string) ([]os.FileInfo, error)
	Stat(name string) (os.FileInfo, error)
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

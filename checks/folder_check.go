package checks

import (
	"fmt"
	"os"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
)

// Folder test error types are stable identifiers exposed to templates and custom metrics.
const (
	folderMinAgeError        = "min_age"
	folderMaxAgeError        = "max_age"
	folderMinCountError      = "min_count"
	folderMaxCountError      = "max_count"
	folderAvailableSizeError = "available_size"
	folderTotalSizeError     = "total_size"
	folderMinSizeError       = "min_size"
	folderMaxSizeError       = "max_size"
)

type FolderCheck struct {
	Oldest        *File  `json:"oldest,omitempty"`
	Newest        *File  `json:"newest,omitempty"`
	MinSize       *File  `json:"smallest,omitempty"`
	MaxSize       *File  `json:"largest,omitempty"`
	TotalSize     int64  `json:"size,omitempty"`
	AvailableSize int64  `json:"availableSize,omitempty"`
	Files         []File `json:"files"`
}

type File struct {
	Name     string    `json:"name,omitempty"`
	Size     int64     `json:"size,omitempty"`
	Mode     string    `json:"mode,omitempty"`
	Modified time.Time `json:"modified"`
	IsDir    bool      `json:"is_dir,omitempty"`
}

func newFile(file os.FileInfo) *File {
	return &File{
		Name:     file.Name(),
		Size:     file.Size(),
		Mode:     file.Mode().String(),
		Modified: file.ModTime().UTC(),
		IsDir:    file.IsDir(),
	}
}

func (f *FolderCheck) Append(osFile os.FileInfo) {
	file := newFile(osFile)
	if f.Oldest == nil || f.Oldest.Modified.After(osFile.ModTime()) {
		f.Oldest = file
	}
	if f.Newest == nil || f.Newest.Modified.Before(osFile.ModTime()) {
		f.Newest = file
	}
	if f.MinSize == nil || f.MinSize.Size > osFile.Size() {
		f.MinSize = file
	}
	if f.MaxSize == nil || f.MaxSize.Size < osFile.Size() {
		f.MaxSize = file
	}
	f.Files = append(f.Files, *file)
}

func (f FolderCheck) Test(test v1.FolderTest) string {
	_, message := f.test(test)
	return message
}

func (f FolderCheck) test(test v1.FolderTest) (errorType, message string) {
	minAge, err := test.GetMinAge()
	if err != nil {
		return folderMinAgeError, fmt.Sprintf("invalid duration %s: %v", test.MinAge, err)
	}
	maxAge, err := test.GetMaxAge()
	if err != nil {
		return folderMaxAgeError, fmt.Sprintf("invalid duration %s: %v", test.MaxAge, err)
	}

	if test.MinCount != nil && len(f.Files) < *test.MinCount {
		return folderMinCountError, fmt.Sprintf("too few files %d < %d", len(f.Files), *test.MinCount)
	}
	if test.MaxCount != nil && len(f.Files) > *test.MaxCount {
		return folderMaxCountError, fmt.Sprintf("too many files %d > %d", len(f.Files), *test.MaxCount)
	}

	if test.AvailableSize != "" {
		if f.AvailableSize == SizeNotSupported {
			return folderAvailableSizeError, "available size not supported"
		}
		size, err := test.AvailableSize.Value()
		if err != nil {
			return folderAvailableSizeError, fmt.Sprintf("%s is an invalid size: %s", test.AvailableSize, err)
		}
		if f.AvailableSize < *size {
			return folderAvailableSizeError, fmt.Sprintf("available size too small: %v < %v", mb(f.AvailableSize), test.AvailableSize)
		}
	}

	if test.TotalSize != "" {
		if f.TotalSize == SizeNotSupported {
			return folderTotalSizeError, "total size not supported"
		}
		size, err := test.TotalSize.Value()
		if err != nil {
			return folderTotalSizeError, fmt.Sprintf("%s is an invalid size: %s", test.TotalSize, err)
		}
		if f.TotalSize < *size {
			return folderTotalSizeError, fmt.Sprintf("total size too small: %v < %v", mb(f.TotalSize), test.TotalSize)
		}
	}

	if len(f.Files) == 0 {
		// nothing run age/size checks on
		return "", ""
	}
	if minAge != nil && time.Since(f.Newest.Modified) < *minAge {
		return folderMinAgeError, fmt.Sprintf("%s is too new: %s < %s", f.Newest.Name, age(f.Newest.Modified), test.MinAge)
	}
	if maxAge != nil && time.Since(f.Oldest.Modified) > *maxAge {
		return folderMaxAgeError, fmt.Sprintf("%s is too old %s > %s", f.Oldest.Name, age(f.Oldest.Modified), test.MaxAge)
	}

	if test.MinSize != "" {
		size, err := test.MinSize.Value()
		if err != nil {
			return folderMinSizeError, fmt.Sprintf("%s is an invalid size: %s", test.MinSize, err)
		}
		if f.MinSize.Size < *size {
			return folderMinSizeError, fmt.Sprintf("%s is too small: %v < %v", f.MinSize.Name, mb(f.MinSize.Size), test.MinSize)
		}
	}

	if test.MaxSize != "" {
		size, err := test.MaxSize.Value()
		if err != nil {
			return folderMaxSizeError, fmt.Sprintf("%s is an invalid size: %s", test.MaxSize, err)
		}
		if f.MaxSize.Size > *size {
			return folderMaxSizeError, fmt.Sprintf("%s is too large: %v > %v", f.MaxSize.Name, mb(f.MaxSize.Size), test.MaxSize)
		}
	}
	return "", ""
}

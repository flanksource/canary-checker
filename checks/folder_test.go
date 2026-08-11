package checks

import (
	"os"
	"testing"
	"time"

	checkContext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	dutyContext "github.com/flanksource/duty/context"
	. "github.com/onsi/gomega"
)

func TestFolderFilterSinceMath(t *testing.T) {
	RegisterTestingT(t)
	ctx, err := v1.FolderFilter{
		Since: "now-1h",
	}.New()

	Expect(err).ToNot(HaveOccurred())
	Expect(*ctx.Since).To(BeTemporally("~", time.Now().Add(-1*time.Hour), 1*time.Second))
}

func TestFolderFilterSinceParse(t *testing.T) {
	RegisterTestingT(t)
	_, err := v1.FolderFilter{
		Since: "2023-10-31T19:18:57.14974Z",
	}.New()

	Expect(err).ToNot(HaveOccurred())
}

func TestFolderTestErrorTypes(t *testing.T) {
	minCount, maxCount := 2, 1
	now := time.Now()
	newFile := File{Name: "new", Size: 1, Modified: now}
	oldFile := File{Name: "old", Size: 3, Modified: now.Add(-2 * time.Hour)}

	tests := []struct {
		name       string
		folder     FolderCheck
		test       v1.FolderTest
		errorType  string
		wantFailed bool
	}{
		{
			name:       "pass",
			folder:     FolderCheck{},
			test:       v1.FolderTest{},
			wantFailed: false,
		},
		{
			name:       "invalid minimum age",
			folder:     FolderCheck{},
			test:       v1.FolderTest{MinAge: "5x"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "invalid maximum age",
			folder:     FolderCheck{},
			test:       v1.FolderTest{MaxAge: "5x"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "minimum count",
			folder:     FolderCheck{Files: []File{newFile}},
			test:       v1.FolderTest{MinCount: &minCount},
			errorType:  folderMinCountError,
			wantFailed: true,
		},
		{
			name:       "maximum count",
			folder:     FolderCheck{Files: []File{newFile, oldFile}},
			test:       v1.FolderTest{MaxCount: &maxCount},
			errorType:  folderMaxCountError,
			wantFailed: true,
		},
		{
			name:       "minimum age",
			folder:     FolderCheck{Files: []File{newFile}, Newest: &newFile, Oldest: &newFile},
			test:       v1.FolderTest{MinAge: "1h"},
			errorType:  folderMinAgeError,
			wantFailed: true,
		},
		{
			name:       "maximum age",
			folder:     FolderCheck{Files: []File{oldFile}, Newest: &oldFile, Oldest: &oldFile},
			test:       v1.FolderTest{MaxAge: "1h"},
			errorType:  folderMaxAgeError,
			wantFailed: true,
		},
		{
			name:       "minimum size",
			folder:     FolderCheck{Files: []File{newFile}, Newest: &newFile, Oldest: &newFile, MinSize: &newFile, MaxSize: &newFile},
			test:       v1.FolderTest{MinSize: "2b"},
			errorType:  folderMinSizeError,
			wantFailed: true,
		},
		{
			name:       "maximum size",
			folder:     FolderCheck{Files: []File{oldFile}, Newest: &oldFile, Oldest: &oldFile, MinSize: &oldFile, MaxSize: &oldFile},
			test:       v1.FolderTest{MaxSize: "2b"},
			errorType:  folderMaxSizeError,
			wantFailed: true,
		},
		{
			name:       "available size",
			folder:     FolderCheck{AvailableSize: 1},
			test:       v1.FolderTest{AvailableSize: "2b"},
			errorType:  folderAvailableSizeError,
			wantFailed: true,
		},
		{
			name:       "total size",
			folder:     FolderCheck{TotalSize: 1},
			test:       v1.FolderTest{TotalSize: "2b"},
			errorType:  folderTotalSizeError,
			wantFailed: true,
		},
		{
			name:       "available size not supported",
			folder:     FolderCheck{AvailableSize: SizeNotSupported},
			test:       v1.FolderTest{AvailableSize: "2b"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "invalid available size",
			folder:     FolderCheck{AvailableSize: 1},
			test:       v1.FolderTest{AvailableSize: "5x"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "total size not supported",
			folder:     FolderCheck{TotalSize: SizeNotSupported},
			test:       v1.FolderTest{TotalSize: "2b"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "invalid total size",
			folder:     FolderCheck{TotalSize: 1},
			test:       v1.FolderTest{TotalSize: "5x"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "invalid minimum size",
			folder:     FolderCheck{Files: []File{newFile}, MinSize: &newFile, MaxSize: &newFile},
			test:       v1.FolderTest{MinSize: "5x"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
		{
			name:       "invalid maximum size",
			folder:     FolderCheck{Files: []File{newFile}, MinSize: &newFile, MaxSize: &newFile},
			test:       v1.FolderTest{MaxSize: "5x"},
			errorType:  folderConfigurationError,
			wantFailed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorType, message := tt.folder.test(tt.test)
			if errorType != tt.errorType {
				t.Errorf("errorType = %q, want %q", errorType, tt.errorType)
			}
			if (message != "") != tt.wantFailed {
				t.Errorf("message = %q, wantFailed = %v", message, tt.wantFailed)
			}
			if got := tt.folder.Test(tt.test); got != message {
				t.Errorf("Test() = %q, want %q", got, message)
			}
		})
	}
}

func TestFolderResultErrorTypeIsAvailableToTemplates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/one.txt", []byte("test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	minCount := 2
	check := v1.FolderCheck{
		Description: v1.Description{Name: "folder"},
		Path:        dir,
		FolderTest:  v1.FolderTest{MinCount: &minCount},
	}
	ctx := checkContext.New(dutyContext.New(), v1.Canary{})
	results := checkLocalFolder(ctx, check)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Pass {
		t.Fatal("folder result passed, want failure")
	}
	if got := results[0].Data["errorType"]; got != "min_count" {
		t.Fatalf("errorType = %#v, want min_count", got)
	}

	value, err := template(ctx.WithCheckResult(results[0]), v1.Template{
		Expression: `pass ? "none" : errorType`,
	})
	if err != nil {
		t.Fatalf("failed to evaluate errorType: %v", err)
	}
	if value != "min_count" {
		t.Errorf("template value = %q, want min_count", value)
	}
	errorValue, err := template(ctx.WithCheckResult(results[0]), v1.Template{Expression: "error"})
	if err != nil {
		t.Fatalf("failed to evaluate error: %v", err)
	}
	if errorValue != results[0].Error {
		t.Errorf("error template value = %q, want %q", errorValue, results[0].Error)
	}

	minCount = 1
	passingResults := checkLocalFolder(ctx, check)
	if !passingResults[0].Pass {
		t.Fatalf("folder result failed, want pass: %s", passingResults[0].Error)
	}
	value, err = template(ctx.WithCheckResult(passingResults[0]), v1.Template{
		Expression: `pass ? "none" : errorType`,
	})
	if err != nil {
		t.Fatalf("failed to evaluate passing errorType: %v", err)
	}
	if value != "none" {
		t.Errorf("passing template value = %q, want none", value)
	}
}

func TestErrorTypeIsAvailableToTemplatesForGenericResults(t *testing.T) {
	check := v1.FolderCheck{Description: v1.Description{Name: "folder"}}
	canary := v1.Canary{}
	tests := []struct {
		name   string
		result *pkg.CheckResult
		want   string
	}{
		{name: "success", result: pkg.Success(check, canary), want: "none"},
		{name: "invalid", result: pkg.Invalid(check, canary, "invalid retries: boom")[0], want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := checkContext.New(dutyContext.New(), canary).WithCheckResult(tt.result)
			value, err := template(ctx, v1.Template{Expression: `pass ? "none" : errorType`})
			if err != nil {
				t.Fatalf("failed to evaluate errorType: %v", err)
			}
			if value != tt.want {
				t.Errorf("template value = %q, want %q", value, tt.want)
			}
		})
	}
}

func TestFolderInfrastructureErrorTypes(t *testing.T) {
	ctx := checkContext.New(dutyContext.New(), v1.Canary{})

	connectionResults := CheckGCSBucket(ctx, v1.FolderCheck{
		Description: v1.Description{Name: "gcs"},
		Path:        "gcs://bucket/path",
	})
	if got := connectionResults[0].Data["errorType"]; got != folderConnectionError {
		t.Errorf("connection errorType = %#v, want %q", got, folderConnectionError)
	}

	listingResults := checkLocalFolder(ctx, v1.FolderCheck{
		Description: v1.Description{Name: "folder"},
		Path:        t.TempDir(),
		Filter:      v1.FolderFilter{Regex: "["},
	})
	if got := listingResults[0].Data["errorType"]; got != folderListingError {
		t.Errorf("listing errorType = %#v, want %q", got, folderListingError)
	}
	if _, ok := listingResults[0].Data["results"].(FolderCheck); !ok {
		t.Errorf("listing results = %#v, want FolderCheck details", listingResults[0].Data["results"])
	}
}

func TestUnsupportedFolderCapacityIsIndependentOfContents(t *testing.T) {
	ctx := checkContext.New(dutyContext.New(), v1.Canary{})
	capacityTests := []struct {
		name string
		test v1.FolderTest
	}{
		{name: "available-size", test: v1.FolderTest{AvailableSize: "2b"}},
		{name: "total-size", test: v1.FolderTest{TotalSize: "2b"}},
	}
	for _, tt := range capacityTests {
		t.Run(tt.name, func(t *testing.T) {
			for _, withFile := range []bool{false, true} {
				name := "empty"
				dir := t.TempDir()
				if withFile {
					name = "non-empty"
					if err := os.WriteFile(dir+"/one.txt", []byte("test"), 0o600); err != nil {
						t.Fatalf("failed to create test file: %v", err)
					}
				}

				t.Run(name, func(t *testing.T) {
					results := checkLocalFolder(ctx, v1.FolderCheck{
						Description: v1.Description{Name: name},
						Path:        dir,
						FolderTest:  tt.test,
					})
					if got := results[0].Data["errorType"]; got != folderConfigurationError {
						t.Errorf("errorType = %#v, want %q", got, folderConfigurationError)
					}
				})
			}
		})
	}
}

//go:build !fast

package checks

import "testing"

func Test_parseS3Path(t *testing.T) {
	tests := []struct {
		name       string
		fullpath   string
		wantBucket string
		wantPath   string
	}{
		{name: "basic", fullpath: "s3://mybucket/developers", wantBucket: "mybucket", wantPath: "developers"},
		{name: "basic", fullpath: "s3://mybucket", wantBucket: "mybucket", wantPath: ""},
		{name: "basic", fullpath: "mybucket", wantBucket: "mybucket", wantPath: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBucket, gotPath := parseS3Path(tt.fullpath)
			if gotBucket != tt.wantBucket {
				t.Errorf("parseS3Path() gotBucket = %v, want %v", gotBucket, tt.wantBucket)
			}
			if gotPath != tt.wantPath {
				t.Errorf("parseS3Path() gotPath = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

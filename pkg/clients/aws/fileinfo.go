//go:build !fast

package aws

import (
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3FileInfo struct {
	Object types.Object
}

func (obj S3FileInfo) Name() string {
	return *obj.Object.Key
}

func (obj S3FileInfo) Size() int64 {
	return obj.Object.Size
}

func (obj S3FileInfo) Mode() fs.FileMode {
	return fs.FileMode(0644)
}

func (obj S3FileInfo) ModTime() time.Time {
	return *obj.Object.LastModified
}

func (obj S3FileInfo) IsDir() bool {
	return strings.HasSuffix(obj.Name(), "/")
}

func (obj S3FileInfo) Sys() interface{} {
	return obj.Object
}

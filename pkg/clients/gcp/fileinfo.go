package gcp

import (
	"io/fs"
	"time"

	gcs "cloud.google.com/go/storage"
)

type GCSFileInfo struct {
	Object *gcs.ObjectAttrs
}

func (GCSFileInfo) IsDir() bool {
	return false
}

func (obj GCSFileInfo) ModTime() time.Time {
	return obj.Object.Updated
}

func (obj GCSFileInfo) Mode() fs.FileMode {
	return fs.FileMode(0644)
}

func (obj GCSFileInfo) Name() string {
	return obj.Object.Name
}

func (obj GCSFileInfo) Size() int64 {
	return obj.Object.Size
}

func (obj GCSFileInfo) Sys() interface{} {
	return obj.Object
}

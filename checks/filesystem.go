package checks

type FilesystemRW interface {
	Filesystem
	Write(name string, data []byte) error
}

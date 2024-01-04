package checks

import (
	"testing"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
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

package api

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	babel "github.com/jvatic/goja-babel"
)

func TestGetCustomRenderer(t *testing.T) {
	babel.Init(4) // Setup 4 transformers (can be any number > 0)
	res, err := babel.Transform(strings.NewReader(`const getCPU = cpu => <span class="cpu-gauge">CPU is {cpu}.</span>`), map[string]interface{}{
		"plugins": []string{
			"transform-react-jsx",
			"transform-block-scoping",
		},
	})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, res)
	fmt.Println("")
}

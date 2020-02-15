package internal

import (
	"github.com/docker/docker/client"
)

var DockerCLI *client.Client

func init() {
	var err error
	DockerCLI, err = client.NewEnvClient()
	if err != nil {
		panic(err)
	}
}
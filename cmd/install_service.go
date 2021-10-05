package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var InstallService = &cobra.Command{
	Use:   "install-service",
	Short: "Install Canary Checker as a Service",
	Run:   installService,
}

var serviceLogger service.Logger

type program struct{}

var ServiceConfig = &service.Config{
	Name:        "canary-checker",
	DisplayName: "Canary Checker Server",
	Description: "Starts the canary checker server",
}

func (p *program) Start(s service.Service) error {
	p.run()
	return nil
}
func (p *program) run() {
	serverRun(nil, nil)
}
func (p *program) Stop(s service.Service) error {
	return nil
}

func installService(cmd *cobra.Command, args []string) {
	path, err := os.Executable()
	if err != nil {
		serviceLogger.Error(err) // nolint: errcheck
		return
	}
	configFile = filepath.Join(filepath.Dir(path), configFile)
	prg := &program{}
	ServiceConfig.Arguments = getArguments()
	s, err := service.New(prg, ServiceConfig)
	if err != nil {
		serviceLogger.Error(err) // nolint: errcheck
		return
	}
	serviceLogger, err = s.Logger(nil)
	if err != nil {
		serviceLogger.Error(err) // nolint: errcheck
		return
	}
	err = s.Install()
	if err != nil {
		serviceLogger.Warning(err) // nolint: errcheck
		return
	}

	serviceLogger.Info("Service Installed Successfully.") // nolint: errcheck
}

func getArguments() []string {
	arguments := []string{"serve", "--configfile"}
	if configFile != "" {
		arguments = append(arguments, configFile)
	} else {
		arguments = append(arguments, "canary-checker.yaml")
	}
	if httpPort != 0 {
		arguments = append(arguments, "--httpPort", fmt.Sprint(httpPort))
	}
	if metricsPort != 0 {
		arguments = append(arguments, "--metricsPort", fmt.Sprint(metricsPort))
	}
	if !logFail {
		arguments = append(arguments, "--logFail=false")
	}
	if logPass {
		arguments = append(arguments, "--logPass")
	}
	if namespace != "" {
		arguments = append(arguments, "--namespace", namespace)
	}
	if includeCheck != "" {
		arguments = append(arguments, "--include-check", includeCheck)
	}
	if cache.Size != 0 {
		arguments = append(arguments, "--maxStatusCheckCount", fmt.Sprint(cache.Size))
	}
	if len(pushServers) > 0 {
		servers := ""
		for _, server := range pushServers {
			servers += server + ","
		}
		arguments = append(arguments, "--push-servers", servers)
	}
	if runner.RunnerName != "" {
		arguments = append(arguments, "--name", runner.RunnerName)
	}
	return arguments
}

func init() {
	InstallService.Flags().StringVarP(&configFile, "configfile", "c", "canary-checker.yaml", "Path to the config file")
	CommonFlags(InstallService.Flags())
}

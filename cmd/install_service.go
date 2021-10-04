package cmd

import (
	"os"
	"path/filepath"

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
		serviceLogger.Error(err)
	}
	path = filepath.Join(filepath.Dir(path), configFile)
	prg := &program{}
	ServiceConfig.Arguments = []string{"serve", "--configfile", path}
	s, err := service.New(prg, ServiceConfig)
	if err != nil {
		serviceLogger.Error(err)
		return
	}
	serviceLogger, err = s.Logger(nil)
	if err != nil {
		serviceLogger.Error(err)
		return
	}
	err = s.Install()
	if err != nil {
		serviceLogger.Warning(err)
		return
	}

	serviceLogger.Info("Service Installed Successfully.")
}

func init() {
	InstallService.Flags().StringVarP(&configFile, "config", "c", "canary-checker.yaml", "Path to the config file")
}

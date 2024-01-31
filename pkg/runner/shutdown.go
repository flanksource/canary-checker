package runner

import (
	"os"

	"github.com/flanksource/commons/logger"
)

var shutdownHooks []func()

func Shutdown() {
	if len(shutdownHooks) == 0 {
		return
	}
	logger.Infof("Shutting down")
	for _, fn := range shutdownHooks {
		fn()
	}
	shutdownHooks = []func(){}
}

func ShutdownAndExit(code int, msg string) {
	Shutdown()
	logger.StandardLogger().WithSkipReportLevel(1).Errorf(msg)
	os.Exit(code)
}

func AddShutdownHook(fn func()) {
	shutdownHooks = append(shutdownHooks, fn)
}

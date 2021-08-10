package pkg

import (
	"bufio"
	"os"
	"strings"

	"github.com/flanksource/commons/logger"
)

var RunnerLabels map[string]string = make(map[string]string)

func LoadLabels(path string) map[string]string {
	result := make(map[string]string)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No label metadata mounted into the operator pod
		logger.Infof("No label file mounted at %s", path)
		return result
	}
	f, err := os.Open(path)
	if err != nil {
		logger.Errorf("Failed to read label file (%s): %v", path, err)
		return result
	}
	defer func() {
		if err = f.Close(); err != nil {
			logger.Errorf("Failed to close label file (%s): %v", path, err)
		}
	}()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.Split(s.Text(), "=")
		result[line[0]] = line[1]
	}
	err = s.Err()
	if err != nil {
		logger.Errorf("Failed to read label file (%s): %v", path, err)
	}

	return result
}

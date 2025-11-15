package labels

import (
	"bufio"
	"os"
	"strings"

	"github.com/flanksource/commons/logger"
)

var IgnoreLabels = []string{
	"pod-template-hash",
	"kustomize.toolkit.fluxcd.io",
}

func FilterLabels(labels map[string]string) map[string]string {
	var new = make(map[string]string)
outer:
	for k, v := range labels {
		for _, ignore := range IgnoreLabels {
			if strings.HasPrefix(k, ignore) {
				continue outer
			}
		}
		new[k] = v
	}
	return new
}

func LoadFromFile(path string) map[string]string {
	result := make(map[string]string)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No label metadata mounted into the operator pod
		logger.Tracef("No label file mounted at %s", path)
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

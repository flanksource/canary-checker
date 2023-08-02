package utils

import (
	"bufio"
	"os"
	"strings"

	"github.com/flanksource/commons/logger"
)

type Properties map[string]string

func ParsePropertiesFile(filename string) (Properties, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	props := Properties{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 {
			logger.Warnf("invalid line: %s", line)
			continue
		}

		key := strings.TrimSpace(tokens[0])
		value := strings.TrimSpace(tokens[1])
		props[key] = value
	}

	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	return props, nil
}

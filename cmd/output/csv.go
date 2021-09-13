package output

import (
	"strconv"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/jszwec/csvutil"
)

type CSVResult struct {
	Name        string `csv:"name"`
	Namespace   string `csv:"namespace"`
	Endpoint    string `csv:"endpoint"`
	CheckType   string `csv:"checkType"`
	Pass        bool   `csv:"pass"`
	Duration    string `csv:"duration"`
	Description string `csv:"description,omitempty"`
	Message     string `csv:"message,omitempty"`
	Error       string `csv:"error,omitempty"`
	Invalid     bool   `csv:"invalid,omitempty"`
}

func GetCSVReport(checkResults []*pkg.CheckResult) (string, error) {
	var results []CSVResult
	for _, checkResult := range checkResults {
		result := CSVResult{
			Name:        checkResult.Canary.Name,
			Namespace:   checkResult.Canary.Namespace,
			Endpoint:    checkResult.Check.GetEndpoint(),
			CheckType:   checkResult.Check.GetType(),
			Pass:        checkResult.Pass,
			Invalid:     checkResult.Invalid,
			Duration:    strconv.Itoa(int(checkResult.Duration)),
			Description: checkResult.Check.GetDescription(),
			Message:     checkResult.Message,
			Error:       checkResult.Error,
		}
		results = append(results, result)
	}

	csv, err := csvutil.Marshal(results)
	return string(csv), err
}

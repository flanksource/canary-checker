package output

import (
	"strconv"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/jszwec/csvutil"
)

type CSVResult struct {
	CheckType   string `csv:"checkType"`
	Pass        bool   `csv:"pass"`
	Time        string `csv:"time"`
	Description string `csv:"description,omitempty"`
	Message     string `csv:"message,omitempty"`
	Error       string `csv:"error,omitempty"`
	Invalid     bool   `csv:"invalid,omitempty"`
}

func GetCSVReport(checkResults []*pkg.CheckResult) (string, error) {
	var results []CSVResult
	for _, checkResult := range checkResults {
		result := CSVResult{
			CheckType:   checkResult.Check.GetType(),
			Pass:        checkResult.Pass,
			Invalid:     checkResult.Invalid,
			Time:        strconv.Itoa(int(checkResult.Duration)),
			Description: checkResult.Check.GetDescription(),
			Message:     checkResult.Message,
			Error:       checkResult.Error,
		}
		results = append(results, result)
	}

	csv, err := csvutil.Marshal(results)
	return string(csv), err
}

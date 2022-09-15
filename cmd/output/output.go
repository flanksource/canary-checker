package output

import (
	"fmt"
	"os"
)

func HandleOutput(report, outputFile string) error {
	if outputFile != "" {
		err := os.WriteFile(outputFile, []byte(report), 0755)
		if err != nil {
			return err
		}
	} else {
		fmt.Println(report)
	}
	return nil
}

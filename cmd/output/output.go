package output

import (
	"fmt"
	"io/ioutil"
)

func HandleOutput(report, outputFile string) error {
	if outputFile != "" {
		err := ioutil.WriteFile(outputFile, []byte(report), 0755)
		if err != nil {
			return err
		}
	} else {
		fmt.Println(report)
	}
	return nil
}

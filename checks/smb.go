package checks

import (
	"fmt"
	"net"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/text"
	"github.com/hirochachacha/go-smb2"
)

const (
	defaultPort = "445"
)

type SmbChecker struct{}

type SmbStatus struct {
	age   string
	count int
}

func (c *SmbChecker) Type() string {
	return "smb"
}

func (c *SmbChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Smb {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *SmbChecker) Check(extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	smbCheck := extConfig.(v1.SmbCheck)
	template := smbCheck.GetDisplayTemplate()
	var smbStatus SmbStatus
	textResults := smbCheck.GetDisplayTemplate() != ""
	serverPath := getServerPath(smbCheck.Server, smbCheck.Port)
	conn, err := net.Dial("tcp", serverPath)
	if err != nil {
		return smbFailF(smbCheck, textResults, smbStatus, template, "failed getting connection: %v", err)
	}
	defer conn.Close()
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:        smbCheck.Username,
			Password:    smbCheck.Password,
			Domain:      smbCheck.Domain,
			Workstation: smbCheck.Workstation,
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		return smbFailF(smbCheck, textResults, smbStatus, template, "failed connecting to server: %v", err)
	}
	defer s.Logoff() //nolint: errcheck
	fs, err := s.Mount(smbCheck.Sharename)
	if err != nil {
		return smbFailF(smbCheck, textResults, smbStatus, template, "failed mounting sharname %v to server: %v", smbCheck.Sharename, err)
	}
	defer fs.Umount() //nolint: errcheck
	age, count, err := getLatestFileAgeAndCount(fs)
	if err != nil {
		return smbFailF(smbCheck, textResults, smbStatus, template, "error traversing files: %v", err)
	}
	smbStatus.age = age.String()
	smbStatus.count = count
	minAge, err := time.ParseDuration(smbCheck.MinAge)
	if err != nil {
		return smbFailF(smbCheck, textResults, smbStatus, template, "error parsing minAge: %v", err)
	}
	if age < minAge {
		return smbFailF(smbCheck, textResults, smbStatus, template, "age of latest object %v is less than the minimum age: %v ", age, minAge)
	}
	if count < smbCheck.MinCount {
		return smbFailF(smbCheck, textResults, smbStatus, template, "file count: %v is less than specified minCount: %v", count, smbCheck.MinCount)
	}
	var results = map[string]interface{}{"age": smbStatus.age, "count": smbStatus.count}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		return smbFailF(smbCheck, textResults, smbStatus, template, "error templating the message: %v", err)
	}
	return Successf(smbCheck, start, textResults, message)
}

func getServerPath(url string, port int) string {
	if port != 0 {
		return fmt.Sprintf("%s:%d", url, port)
	}
	return url + ":" + defaultPort
}

func getLatestFileAgeAndCount(fs *smb2.Share) (duration time.Duration, count int, err error) {
	files, err := fs.ReadDir(".")
	if err != nil {
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if duration < time.Since(file.ModTime()) {
			duration = time.Since(file.ModTime())
		}
		count++
	}
	return
}

func smbFailF(check external.Check, textResults bool, smbStatus SmbStatus, template, msg string, args ...interface{}) *pkg.CheckResult {
	var results = map[string]interface{}{"age": smbStatus.age, "count": smbStatus.count}
	message := smbTemplateResult(template, results)
	message = message + "\n" + fmt.Sprintf(msg, args...)
	return TextFailf(check, textResults, message)
}

func smbTemplateResult(template string, results map[string]interface{}) (message string) {
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		message = message + "\n" + err.Error()
	}
	return message
}

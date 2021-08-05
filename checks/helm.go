package checks

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/flanksource/kommons"

	pusher "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
)

type HelmChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *HelmChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

type ResultWriter struct{}

// Type: returns checker type
func (c *HelmChecker) Type() string {
	return "helm"
}

func (c *HelmChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Helm {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *HelmChecker) Check(extConfig external.Check) *pkg.CheckResult {
	config := extConfig.(v1.HelmCheck)
	start := time.Now()
	var uploadOK, downloadOK = true, true
	chartmuseum := fmt.Sprintf("%s/chartrepo/%s/", config.Chartmuseum, config.Project)
	logger.Tracef("Uploading test chart")
	var username, password string
	var err error
	namespace := config.GetNamespace()
	if config.Auth != nil {
		_, username, err = c.kommons.GetEnvValue(config.Auth.Username, namespace)
		if err != nil {
			return Failf(config, "failed to get username: %v", err)
		}
		_, password, err = c.kommons.GetEnvValue(config.Auth.Password, namespace)
		if err != nil {
			return Failf(config, "failed to get password: %v", err)
		}
	}
	client, _ := pusher.NewClient(
		pusher.URL(chartmuseum),
		pusher.Username(username),
		pusher.Password(password),
		pusher.ContextPath(""),
		pusher.Timeout(60),
		pusher.CAFile(*config.CaFile))
	chartPath, err := createTestChart()
	if err != nil {
		return &pkg.CheckResult{
			Pass:     false,
			Check:    config,
			Invalid:  true,
			Duration: 0,
			Message:  fmt.Sprintf("Failed to create test chart: %v", err),
		}
	}
	response, err := client.UploadChartPackage(*chartPath, false)
	defer func() {
		response.Close = true
	}()
	if err != nil {
		return &pkg.CheckResult{
			Check:    config,
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Message:  fmt.Sprintf("Failed to check: %v", err),
		}
	}

	if response.StatusCode != 201 {
		return &pkg.CheckResult{
			Check:    config,
			Pass:     false,
			Invalid:  false,
			Duration: 0,
			Message:  "Failed to push test chart",
		}
	}

	if err != nil {
		return &pkg.CheckResult{
			Check:    config,
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Message:  fmt.Sprintf("Failed to get user: %v", err),
		}
	}

	defer os.RemoveAll("./test-chart-0.1.0.tgz") // nolint: errcheck

	iCli := action.NewPull()
	if config.CaFile != nil {
		iCli.CaFile = *config.CaFile
	}
	kubeconfigPath := pkg.GetKubeconfig()
	iCli.Settings = &cli.EnvSettings{
		KubeConfig: kubeconfigPath,
	}

	logger.Tracef("Pulling test chart")
	url, err := url.Parse(chartmuseum)
	if err != nil {
		return &pkg.CheckResult{
			Check:    config,
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Message:  fmt.Sprintf("Failed to parse chartmuseum url: %v", err),
		}
	}
	url.Path = path.Join(url.Path, "charts/test-chart-0.1.0.tgz")
	_, err = iCli.Run(url.String())
	if err != nil {
		return &pkg.CheckResult{
			Check:    config,
			Pass:     false,
			Invalid:  false,
			Duration: 0,
		}
	}

	defer cleanUp("test-chart", chartmuseum, config, username, password) // nolint: errcheck

	if err != nil {
		logger.Warnf("Failed to perform cleanup: %v", err)
	}
	elapsed := time.Since(start)
	return &pkg.CheckResult{
		Check:    config,
		Pass:     uploadOK && downloadOK,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
	}
}

func cleanUp(chartname string, chartmuseum string, config v1.HelmCheck, username, password string) error {
	caCert, err := ioutil.ReadFile(*config.CaFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	url, err := url.Parse(chartmuseum)
	if err != nil {
		return fmt.Errorf("failed to parse chartmuseum url: %v", err)
	}
	url.Path = path.Join("api", url.Path, "charts", chartname)
	req, err := http.NewRequest("DELETE", url.String(), nil)
	req.SetBasicAuth(username, password)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get http client: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete test chart. Error code: %d", resp.StatusCode)
	}
	return nil
}

func createTestChart() (*string, error) {
	dir, err := ioutil.TempDir("/tmp", "canary_checker_helm_test_chart")
	if err != nil {
		return nil, fmt.Errorf("createTestChart: failed to create temp directory: %v", err)
	}
	chartDir, err := chartutil.Create("test-chart", dir)
	if err != nil {
		return nil, fmt.Errorf("createTestChart: failed to create test chart: %v", err)
	}
	packageAction := action.NewPackage()
	packagePath, err := packageAction.Run(chartDir, make(map[string]interface{}))
	if err != nil {
		return nil, fmt.Errorf("createTestChart: failed to package test chart: %v", err)
	}
	defer os.RemoveAll(dir)
	return &packagePath, nil
}

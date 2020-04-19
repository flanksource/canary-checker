package checks

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	pusher "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/flanksource/canary-checker/pkg"
	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

type HelmChecker struct{}

type ResultWriter struct {}

// Type: returns checker type
func (c *HelmChecker) Type() string {
	return "helm"
}

func (c *HelmChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.Helm {
		results <- c.Check(conf.HelmCheck)
	}
}

func (c *HelmChecker) Check(config pkg.HelmCheck) *pkg.CheckResult {
	start := time.Now()
	var uploadOK, downloadOK bool = true, true
	chartmuseum := fmt.Sprintf("%s/chartrepo/%s/", config.Chartmuseum, config.Project)
	log.Trace("Uploading test chart")
	client, _  := pusher.NewClient(
		pusher.URL(chartmuseum),
		pusher.Username(config.Username),
		pusher.Password(config.Password),
		pusher.ContextPath(""),
		pusher.Timeout(60),
		pusher.CAFile(*config.CaFile),)
	chartPath, err := createTestChart()
	if err != nil {
		return &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Endpoint: config.Chartmuseum,
			Message:  fmt.Sprintf("Failed to create test chart: %v", err),
			Metrics:  getHelmMetrics(config, false),
		}
	}
	response, err := client.UploadChartPackage(*chartPath, false)

	if err != nil {
		return &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Endpoint: config.Chartmuseum,
			Message:  fmt.Sprintf("Failed to check: %v", err),
			Metrics:  getHelmMetrics(config, false),
		}
	}
	log.Trace(response)
	log.Trace(response.Request)

	if response.StatusCode != 201 {
		uploadOK = false
		return &pkg.CheckResult{
			Pass:     false,
			Invalid:  false,
			Duration: 0,
			Endpoint: config.Chartmuseum,
			Message:  "Failed to push test chart",
			Metrics:  getHelmMetrics(config, false),
		}
	}

	if err != nil {
		return &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Endpoint: config.Chartmuseum,
			Message:  fmt.Sprintf("Failed to get user: %v", err),
			Metrics:  getHelmMetrics(config, false),
		}
	}

	defer os.RemoveAll("./test-chart-0.1.0.tgz")

	iCli := action.NewPull()
	if config.CaFile != nil {
		iCli.CaFile = *config.CaFile
	}
	kubeconfigPath := pkg.GetKubeconfig()
	iCli.Settings = &cli.EnvSettings{
		KubeConfig:       kubeconfigPath,
	}

	log.Trace("Pulling test chart")
	url, err := url.Parse(chartmuseum)
	if err != nil {
		return &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Duration: 0,
			Endpoint: config.Chartmuseum,
			Message:  fmt.Sprintf("Failed to parse chartmuseum url: %v", err),
			Metrics:  getHelmMetrics(config, false),
		}
	}
	url.Path = path.Join(url.Path, "charts/test-chart-0.1.0.tgz")
	_, err = iCli.Run(url.String())
	if err != nil {
		log.Trace(err)
		downloadOK = false
		return &pkg.CheckResult{
			Pass:     false,
			Invalid:  false,
			Duration: 0,
			Endpoint: config.Chartmuseum,
			Message:  "Failed to pull test chart",
			Metrics:  getHelmMetrics(config, false),
		}
	}

	defer cleanUp("test-chart", chartmuseum, config)

	if err != nil {
		log.Warnf("Failed to perform cleanup: %v", err)
	}
	elapsed := time.Since(start)
	return &pkg.CheckResult{
		Pass:     uploadOK && downloadOK,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Endpoint: config.Chartmuseum,
		Message:  "Successful push and pull",
		Metrics:  getHelmMetrics(config, uploadOK && downloadOK),
	}
}

func cleanUp(chartname string, chartmuseum string, config pkg.HelmCheck) error {
	caCert, err := ioutil.ReadFile(*config.CaFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
			},
		},
	}
	url, err := url.Parse(chartmuseum)
	if err != nil {
		return fmt.Errorf("Failed to parse chartmuseum url: %v", err)
	}
	url.Path = path.Join("api", url.Path, "charts", chartname)
	req, err := http.NewRequest("DELETE", url.String(), nil)
	req.SetBasicAuth(config.Username, config.Password)
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

func getHelmMetrics(check pkg.HelmCheck, pass bool) []pkg.Metric {
	var value float64 = 0
	if pass {
		value = 1
	}
	return []pkg.Metric{
		{
			Name: "helm_check_pass",
			Type: pkg.GaugeType,
			Labels: map [string]string{
				"helmCheckProject": check.Project,
				"helmCheckUrl": check.Chartmuseum,
				"helmCheckUsername": check.Username,
			},
			Value: value,
		},
	}
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

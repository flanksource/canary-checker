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

	pusher "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
)

type HelmChecker struct {
}

type ResultWriter struct{}

// Type: returns checker type
func (c *HelmChecker) Type() string {
	return "helm"
}

func (c *HelmChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Helm {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *HelmChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	config := extConfig.(v1.HelmCheck)
	result := pkg.Success(config, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	var uploadOK, downloadOK = true, true
	logger.Tracef("Uploading test chart")
	namespace := ctx.Canary.Namespace
	var err error
	auth, err := GetAuthValues(config.Auth, ctx.Kommons, namespace)
	if err != nil {
		return results.Failf("failed to fetch auth details: %v", err)
	}
	client, _ := pusher.NewClient(
		pusher.URL(config.Chartmuseum),
		pusher.Username(auth.Username.Value),
		pusher.Password(auth.Password.Value),
		pusher.ContextPath(""),
		pusher.Timeout(60),
		pusher.CAFile(config.CaFile))
	chartPath, err := createTestChart()
	if err != nil {
		return results.Failf(err.Error())
	}
	response, err := client.UploadChartPackage(*chartPath, false)
	if err != nil {
		return results.Failf(err.Error())
	}
	defer func() {
		response.Close = true
	}()
	if response.StatusCode != 201 {
		return results.Failf("failed to upload test chart. Error code: %d", response.StatusCode)
	}

	defer os.RemoveAll("./test-chart-0.1.0.tgz") // nolint: errcheck

	iCli := action.NewPull()
	if config.CaFile != "" {
		iCli.CaFile = config.CaFile
	}
	kubeconfigPath := pkg.GetKubeconfig()
	iCli.Settings = &cli.EnvSettings{
		KubeConfig: kubeconfigPath,
	}
	logger.Tracef("Pulling test chart")
	url, err := url.Parse(config.Chartmuseum)
	if err != nil {
		return results.Failf(err.Error())
	}
	url.Path = path.Join(url.Path, "charts/test-chart-0.1.0.tgz")
	_, err = iCli.Run(url.String())
	if err != nil {
		return results.Failf(err.Error())
	}

	defer cleanUp("test-chart", config.Chartmuseum, config, auth.Username.Value, auth.Password.Value) // nolint: errcheck

	result.Pass = uploadOK && downloadOK
	return results
}

func cleanUp(chartname string, chartmuseum string, config v1.HelmCheck, username, password string) error {
	caCert, err := os.ReadFile(config.CaFile)
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

/*
Copyright 2017 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkg

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/flanksource/commons/files"
	clogger "github.com/flanksource/commons/logger"
	"github.com/flanksource/is-healthy/pkg/health"
	"github.com/flanksource/kommons"
	"github.com/henvic/httpretty"
	"github.com/pkg/errors"
	"gopkg.in/flanksource/yaml.v3"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
)

func NewKommonsClientWithConfig(kubeConfig string) (*kommons.Client, kubernetes.Interface, error) {
	getter := func() (*clientcmdapi.Config, error) {
		clientCfg, err := clientcmd.NewClientConfigFromBytes([]byte(kubeConfig))
		if err != nil {
			return nil, err
		}

		apiCfg, err := clientCfg.RawConfig()
		if err != nil {
			return nil, err
		}

		return &apiCfg, nil
	}

	config, err := clientcmd.BuildConfigFromKubeconfigGetter("", getter)
	if err != nil {
		return nil, fake.NewSimpleClientset(), errors.Wrap(err, "Failed to generate rest config")
	}

	return newKommonsClient(config)
}

func NewKommonsClientWithConfigPath(kubeConfigPath string) (*kommons.Client, kubernetes.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fake.NewSimpleClientset(), errors.Wrap(err, "Failed to generate rest config")
	}

	return newKommonsClient(config)
}

func NewKommonsClient() (*kommons.Client, kubernetes.Interface, error) {
	kubeConfig := GetKubeconfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fake.NewSimpleClientset(), errors.Wrap(err, "Failed to generate rest config")
	}

	return newKommonsClient(config)
}

func newKommonsClient(config *rest.Config) (*kommons.Client, kubernetes.Interface, error) {
	if clogger.IsLevelEnabled(7) {
		logger := &httpretty.Logger{
			Time:           true,
			TLS:            clogger.IsLevelEnabled(8),
			RequestHeader:  true,
			RequestBody:    clogger.IsLevelEnabled(9),
			ResponseHeader: true,
			ResponseBody:   clogger.IsLevelEnabled(8),
			Colors:         true, // erase line if you don't like colors
			Formatters:     []httpretty.Formatter{&httpretty.JSONFormatter{}},
		}

		config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return logger.RoundTripper(rt)
		}
	}

	Client := kommons.NewClient(config, clogger.StandardLogger())
	if Client == nil {
		return nil, fake.NewSimpleClientset(), errors.New("could not create kommons client")
	}

	k8s, err := Client.GetClientset()
	if err == nil {
		return Client, k8s, nil
	}
	return nil, fake.NewSimpleClientset(), errors.Wrap(err, "failed to create k8s client")
}

func GetClusterName(config *rest.Config) string {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return ""
	}
	kubeadmConfig, err := clientset.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "kubeadm-config", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	clusterConfiguration := make(map[string]interface{})

	if err := yaml.Unmarshal([]byte(kubeadmConfig.Data["ClusterConfiguration"]), &clusterConfiguration); err != nil {
		return ""
	}
	return clusterConfiguration["clusterName"].(string)
}

func GetKubeconfig() string {
	var kubeConfig string
	if os.Getenv("KUBECONFIG") != "" {
		kubeConfig = os.Getenv("KUBECONFIG")
	} else if home := homedir.HomeDir(); home != "" {
		kubeConfig = filepath.Join(home, ".kube", "config")
		if !files.Exists(kubeConfig) {
			kubeConfig = ""
		}
	}
	return kubeConfig
}

// kubeClient is an updated & stripped of kommons client
type kubeClient struct {
	restMapper    *restmapper.DeferredDiscoveryRESTMapper
	dynamicClient *dynamic.DynamicClient
	GetRESTConfig func() (*rest.Config, error)
}

func NewKubeClient(restConfigFn func() (*rest.Config, error)) *kubeClient {
	return &kubeClient{GetRESTConfig: restConfigFn}
}

func (c *kubeClient) WaitForResource(ctx context.Context, kind, namespace, name string) (*health.HealthStatus, error) {
	client, err := c.GetClientByKind(kind)
	if err != nil {
		return nil, err
	}

	for {
		item, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting item (kind=%s, namespace=%s, name=%s)", kind, namespace, name)
		}

		status, err := health.GetResourceHealth(item, nil)
		if err != nil {
			return nil, fmt.Errorf("error getting resource health: %w", err)
		}

		if status.Status == health.HealthStatusHealthy {
			return status, nil
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *kubeClient) GetClientByKind(kind string) (dynamic.NamespaceableResourceInterface, error) {
	dynamicClient, err := c.GetDynamicClient()
	if err != nil {
		return nil, err
	}

	rm, _ := c.GetRestMapper()
	gvk, err := rm.KindFor(schema.GroupVersionResource{
		Resource: kind,
	})
	if err != nil {
		return nil, err
	}

	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(mapping.Resource), nil
}

// GetDynamicClient creates a new k8s client
func (c *kubeClient) GetDynamicClient() (dynamic.Interface, error) {
	if c.dynamicClient != nil {
		return c.dynamicClient, nil
	}

	cfg, err := c.GetRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("getClientset: failed to get REST config: %v", err)
	}
	c.dynamicClient, err = dynamic.NewForConfig(cfg)
	return c.dynamicClient, err
}

func (c *kubeClient) GetRestMapper() (meta.RESTMapper, error) {
	if c.restMapper != nil {
		return c.restMapper, nil
	}

	config, _ := c.GetRESTConfig()

	// re-use kubectl cache
	host := config.Host
	host = strings.ReplaceAll(host, "https://", "")
	host = strings.ReplaceAll(host, "-", "_")
	host = strings.ReplaceAll(host, ":", "_")
	cacheDir := os.ExpandEnv("$HOME/.kube/cache/discovery/" + host)
	cache, err := disk.NewCachedDiscoveryClientForConfig(config, cacheDir, "", 10*time.Minute)
	if err != nil {
		return nil, err
	}
	c.restMapper = restmapper.NewDeferredDiscoveryRESTMapper(cache)
	return c.restMapper, err
}

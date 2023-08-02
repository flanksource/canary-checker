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
	"os"
	"path/filepath"

	"github.com/flanksource/commons/logger"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/flanksource/commons/files"
	"github.com/flanksource/kommons"
	"github.com/pkg/errors"
	"gopkg.in/flanksource/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
)

func NewKommonsClient() (*kommons.Client, kubernetes.Interface, error) {
	kubeConfig := GetKubeconfig()
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fake.NewSimpleClientset(), errors.Wrap(err, "Failed to generate rest config")
	}
	Client := kommons.NewClient(config, logger.StandardLogger())
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

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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/flanksource/commons/files"
	"github.com/flanksource/kommons"
	"github.com/pkg/errors"
	"gopkg.in/flanksource/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
)

func NewK8sClient() (*kubernetes.Clientset, error) {
	kubeConfig := GetKubeconfig()
	configBytes, err := ioutil.ReadFile(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Could not read kubernetes config")
	}
	Client, err := kommons.NewClientFromBytes(configBytes)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kommons wrapper")
	}
	clientset, err := Client.GetClientset()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create k8s client")
	}

	return clientset, nil
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
	fmt.Printf("KUBECONFIG: %v\n", os.Getenv("KUBECONFIG"))
	if os.Getenv("KUBECONFIG") != "" {
		kubeConfig = os.Getenv("KUBECONFIG")
	} else if home := homedir.HomeDir(); home != "" {
		kubeConfig = filepath.Join(home, ".kube", "config")
		fmt.Printf("Checking file %v...", kubeConfig)
		if !files.Exists(kubeConfig) {
			fmt.Println("failed")
			kubeConfig = ""
		} else {
			fmt.Println("found")
		}
	}
	return kubeConfig
}

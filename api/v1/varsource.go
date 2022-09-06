package v1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

// VarSource represents a source for a value
type VarSource struct {
	// Selects a field of the pod: supports metadata.name, metadata.namespace, metadata.labels, metadata.annotations,
	// spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.
	// +optional
	FieldRef *corev1.ObjectFieldSelector `json:"fieldRef,omitempty" protobuf:"bytes,1,opt,name=fieldRef"`
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty" protobuf:"bytes,3,opt,name=configMapKeyRef"`
	// Selects a key of a secret in the pod's namespace
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty" protobuf:"bytes,4,opt,name=secretKeyRef"`
}

// getSecretRefValue returns the value of a secret in the supplied namespace
func getSecretRefValue(client kubernetes.Interface, namespace string, secretSelector *corev1.SecretKeySelector) (string, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretSelector.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if data, ok := secret.Data[secretSelector.Key]; ok {
		return string(data), nil
	}
	return "", fmt.Errorf("key %s not found in secret %s", secretSelector.Key, secretSelector.Name)
}

// getConfigMapRefValue returns the value of a configmap in the supplied namespace
func getConfigMapRefValue(client kubernetes.Interface, namespace string, configMapSelector *corev1.ConfigMapKeySelector) (string, error) {
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapSelector.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if data, ok := configMap.Data[configMapSelector.Key]; ok {
		return data, nil
	}
	return "", fmt.Errorf("key %s not found in config map %s", configMapSelector.Key, configMapSelector.Name)
}

// getFieldRef returns the value of the supplied path in the given object
func getFieldRef(obj runtime.Object, from *VarSource) (string, error) {
	return extractFieldPathAsString(obj, from.FieldRef.FieldPath)
}

// extractFieldPathAsString extracts the field from the given object
// and returns it as a string.  The object must be a pointer to an
// API type.
func extractFieldPathAsString(obj interface{}, fieldPath string) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", nil
	}

	if path, subscript, ok := splitMaybeSubscriptedPath(fieldPath); ok {
		switch path {
		case "metadata.annotations":
			if errs := validation.IsQualifiedName(strings.ToLower(subscript)); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetAnnotations()[subscript], nil
		case "metadata.labels":
			if errs := validation.IsQualifiedName(subscript); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetLabels()[subscript], nil
		default:
			return "", fmt.Errorf("fieldPath %q does not support subscript", fieldPath)
		}
	}

	switch fieldPath {
	case "metadata.annotations":
		return formatMap(accessor.GetAnnotations()), nil
	case "metadata.labels":
		return formatMap(accessor.GetLabels()), nil
	case "metadata.name":
		return accessor.GetName(), nil
	case "metadata.namespace":
		return accessor.GetNamespace(), nil
	case "metadata.uid":
		return string(accessor.GetUID()), nil
	}

	return "", fmt.Errorf("unsupported fieldPath: %v", fieldPath)
}

// splitMaybeSubscriptedPath checks whether the specified fieldPath is
// subscripted, and
//   - if yes, this function splits the fieldPath into path and subscript, and
//     returns (path, subscript, true).
//   - if no, this function returns (fieldPath, "", false).
//
// Example inputs and outputs:
//   - "metadata.annotations['myKey']" --> ("metadata.annotations", "myKey", true)
//   - "metadata.annotations['a[b]c']" --> ("metadata.annotations", "a[b]c", true)
//   - "metadata.labels[”]"           --> ("metadata.labels", "", true)
//   - "metadata.labels"               --> ("metadata.labels", "", false)
func splitMaybeSubscriptedPath(fieldPath string) (string, string, bool) {
	if !strings.HasSuffix(fieldPath, "']") {
		return fieldPath, "", false
	}
	s := strings.TrimSuffix(fieldPath, "']")
	parts := strings.SplitN(s, "['", 2)
	if len(parts) < 2 {
		return fieldPath, "", false
	}
	if len(parts[0]) == 0 {
		return fieldPath, "", false
	}
	return parts[0], parts[1], true
}

// formatMap formats map[string]string to a string.
func formatMap(m map[string]string) (fmtStr string) {
	// output with keys in sorted order to provide stable output
	keys := sets.NewString()
	for key := range m {
		keys.Insert(key)
	}
	for _, key := range keys.List() {
		fmtStr += fmt.Sprintf("%v=%q\n", key, m[key])
	}
	fmtStr = strings.TrimSuffix(fmtStr, "\n")

	return
}

// GetEnvVarRefValue returns the value referenced by the supplied EnvVarSource given the other supplied information.
func GetEnvVarRefValue(kc kubernetes.Interface, ns string, from *VarSource, obj runtime.Object) (string, error) {
	if from.Value != "" {
		return from.Value, nil
	}
	if from.SecretKeyRef != nil {
		return getSecretRefValue(kc, ns, from.SecretKeyRef)
	}

	if from.ConfigMapKeyRef != nil {
		return getConfigMapRefValue(kc, ns, from.ConfigMapKeyRef)
	}

	if from.FieldRef != nil {
		return getFieldRef(obj, from)
	}

	return "", fmt.Errorf("invalid valueFrom: #%v", *from)
}

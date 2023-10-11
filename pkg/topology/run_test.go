package topology

import (
	"os"

	v1 "github.com/flanksource/canary-checker/api/v1"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func yamlFileToTopology(file string) (t v1.Topology, err error) {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(fileContent, &obj)
	if err != nil {
		return
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &t)
	if err != nil {
		return
	}
	return
}

var _ = ginkgo.Describe("Test topology run", ginkgo.Ordered, func() {
	opts := TopologyRunOptions{
		Client:     nil,
		Kubernetes: nil,
		Depth:      10,
		Namespace:  "default",
	}

	ginkgo.It("should create component with properties", func() {
		t, err := yamlFileToTopology("../../fixtures/topology/component-with-properties.yml")
		if err != nil {
			ginkgo.Fail("Error converting yaml to v1.Topology:" + err.Error())
		}

		rootComponent := Run(opts, t)
		Expect(len(rootComponent[0].Components)).To(Equal(3))

		componentA := rootComponent[0].Components[0]
		componentB := rootComponent[0].Components[1]
		componentC := rootComponent[0].Components[2]

		Expect(string(componentA.Properties.AsJSON())).To(MatchJSON(`[{"name":"error_percentage","value":1,"min":0,"max":100},{"name":"owner","text":"team-a"},{"name":"company","text":"Acme"},{"name":"location","text":"Mars"}]`))
		Expect(string(componentB.Properties.AsJSON())).To(MatchJSON(`[{"name":"error_percentage","value":10,"min":0,"max":100},{"name":"owner","text":"team-b"},{"name":"company","text":"Acme"},{"name":"location","text":"Mars"}]`))
		Expect(string(componentC.Properties.AsJSON())).To(MatchJSON(`[{"name":"error_percentage","value":50,"min":0,"max":100},{"name":"owner","text":"team-b"},{"name":"company","text":"Acme"},{"name":"location","text":"Mars"}]`))
	})
})

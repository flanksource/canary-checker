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

	ginkgo.It("should create component with forEach functionality", func() {
		t, err := yamlFileToTopology("../../fixtures/topology/component-with-for-each.yml")
		if err != nil {
			ginkgo.Fail("Error converting yaml to v1.Topology:" + err.Error())
		}

		rootComponent := Run(opts, t)
		Expect(len(rootComponent[0].Components)).To(Equal(3))

		componentA := rootComponent[0].Components[0]
		componentB := rootComponent[0].Components[1]
		componentC := rootComponent[0].Components[2]

		// Test correct merging of properties
		Expect(string(componentA.Properties.AsJSON())).To(MatchJSON(`[{"name":"owner","text":"team-a"},{"name":"processor","text":"intel"},{"name":"company","text":"Acme"},{"name":"location","text":"Mars"}]`))
		Expect(string(componentB.Properties.AsJSON())).To(MatchJSON(`[{"name":"owner","text":"team-b"},{"name":"processor","text":"intel"},{"name":"company","text":"Acme"},{"name":"location","text":"Mars"}]`))
		Expect(string(componentC.Properties.AsJSON())).To(MatchJSON(`[{"name":"owner","text":"team-b"},{"name":"processor","text":"amd"},{"name":"company","text":"Acme"},{"name":"location","text":"Mars"}]`))

		// Each component should have 2 children named Child-A and Child-B
		Expect(len(componentA.Components)).To(Equal(2))
		Expect(componentA.Components[0].Name).To(Equal("Child-A"))
		Expect(componentA.Components[1].Name).To(Equal("Child-B"))

		Expect(len(componentB.Components)).To(Equal(2))
		Expect(componentB.Components[0].Name).To(Equal("Child-A"))
		Expect(componentB.Components[1].Name).To(Equal("Child-B"))

		Expect(len(componentC.Components)).To(Equal(2))
		Expect(componentC.Components[0].Name).To(Equal("Child-A"))
		Expect(componentC.Components[1].Name).To(Equal("Child-B"))

		// Each component should have a templated config linked
		Expect(len(componentA.Configs)).To(Equal(1))
		Expect(componentA.Configs[0].Name).To(Equal(componentA.Name))
		Expect(componentA.Configs[0].Type).To(Equal("Service"))

		Expect(len(componentB.Configs)).To(Equal(1))
		Expect(componentB.Configs[0].Name).To(Equal(componentB.Name))
		Expect(componentB.Configs[0].Type).To(Equal("Service"))

		Expect(len(componentC.Configs)).To(Equal(1))
		Expect(componentC.Configs[0].Name).To(Equal(componentC.Name))
		Expect(componentC.Configs[0].Type).To(Equal("Service"))
	})
})

package k8s

import (
	"os"
	"path/filepath"
	"testing"

	kt "github.com/siqiliu/kli/internal/types"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// deploymentYAML is a minimal Deployment document for use in undeploy tests.
const deploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`

// writeYAML writes content to a temp file and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatalf("write temp YAML: %v", err)
	}
	return f
}

// makeUndeployClient builds a *Client with fake typed + dynamic clients.
// objects are pre-populated into the dynamic client tracker.
func makeUndeployClient(objects ...runtime.Object) *Client {
	fakeTyped := fake.NewSimpleClientset()
	fd := fakeTyped.Discovery().(*fakediscovery.FakeDiscovery)
	fd.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
			},
		},
	}
	fakeDyn := fakedynamic.NewSimpleDynamicClient(clientgoscheme.Scheme, objects...)
	return &Client{typeClient: fakeTyped, dynamicClient: fakeDyn}
}

func TestUndeploy_SkipsNotFound(t *testing.T) {
	// No objects in the fake cluster — delete should return NotFound → ActionSkipped.
	c := makeUndeployClient()
	f := writeYAML(t, deploymentYAML)

	results, err := c.Undeploy(f, "default")
	if err != nil {
		t.Fatalf("Undeploy() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Action != kt.ActionSkipped {
		t.Errorf("Action = %v, want ActionSkipped", results[0].Action)
	}
	if results[0].Err != nil {
		t.Errorf("Err = %v, want nil", results[0].Err)
	}
}

func TestUndeploy_DeletesExisting(t *testing.T) {
	// Pre-populate the fake cluster with the nginx Deployment.
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
	}
	c := makeUndeployClient(deploy)
	f := writeYAML(t, deploymentYAML)

	results, err := c.Undeploy(f, "default")
	if err != nil {
		t.Fatalf("Undeploy() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Action != kt.ActionDeleted {
		t.Errorf("Action = %v, want ActionDeleted", results[0].Action)
	}
	if results[0].Err != nil {
		t.Errorf("Err = %v, want nil", results[0].Err)
	}
}

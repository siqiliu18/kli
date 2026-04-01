package k8s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client holds two Kubernetes clients serving different purposes:
//
//   - typeClient: typed client for known resource types (Deployments, Pods, etc.)
//     Used by status and logs — status needs typed fields like ReadyReplicas,
//     and logs requires a special streaming HTTP call only available on the typed client.
//
//   - dynamicClient: works with any resource kind without generated types.
//     Used by apply and undeploy — they accept arbitrary YAML (including CRDs)
//     so we can't rely on typed structs that only cover built-in resources.
type Client struct {
	typeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
}

func NewClient() (*Client, error) {
	// NewDefaultClientConfigLoadingRules respects the KUBECONFIG env var and
	// falls back to ~/.kube/config — same precedence as kubectl.
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	typedClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{
		typeClient:    typedClient,
		dynamicClient: dynamicClient,
	}, nil
}

// NamespaceExists checks that namespace exists on the cluster.
// Returns a user-friendly error if not found, so all commands give a clear
// message instead of cryptic API errors or silent empty results.
func (c *Client) NamespaceExists(namespace string) error {
	_, err := c.typeClient.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return fmt.Errorf("namespace %q not found — create it first: kubectl create namespace %s", namespace, namespace)
	}
	return err
}

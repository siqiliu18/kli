package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"

	kt "github.com/siqiliu/kli/internal/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// Undeploy deletes all Kubernetes resources found at path (file or folder) from namespace.
//
// Partial failures are collected and returned — undeploy continues even if one resource fails.
// Resources not found on the cluster are skipped with ActionSkipped (not treated as errors).
func (c *Client) Undeploy(path, namespace string) ([]kt.ResourceResult, error) {
	files, err := collectYAMLFiles(path)
	if err != nil {
		return nil, err
	}

	gr, err := restmapper.GetAPIGroupResources(c.typeClient.Discovery())
	if err != nil {
		return nil, fmt.Errorf("discover API groups: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	var results []kt.ResourceResult

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return results, fmt.Errorf("read %s: %w", file, err)
		}
		for doc := range strings.SplitSeq(string(data), "\n---") {
			doc = strings.TrimSpace(doc)
			if doc == "" {
				continue
			}

			obj := &unstructured.Unstructured{}
			_, gvk, err := decoder.Decode([]byte(doc), nil, obj)
			if err != nil {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("decode: %w", err),
				})
				continue
			}

			mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("map GVK: %w", err),
				})
				continue
			}

			var dr dynamic.ResourceInterface
			if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				dr = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
			} else {
				dr = c.dynamicClient.Resource(mapping.Resource)
			}

			// Delete with empty DeleteOptions — uses cluster defaults (background
			// cascading deletion: owner is removed immediately, owned children
			// like Pods and ReplicaSets are garbage collected asynchronously).
			// IsNotFound → resource already gone, skip gracefully without error.
			err = dr.Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
			if errors.IsNotFound(err) {
				results = append(results, kt.ResourceResult{
					Name:   obj.GetName(),
					Kind:   gvk.Kind,
					Action: kt.ActionSkipped,
				})
				continue
			}
			if err != nil {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("delete: %w", err),
				})
				continue
			}

			// After a successful Delete call, check if the resource is stuck terminating.
			// Kubernetes sets deletionTimestamp immediately but won't remove the resource
			// until all finalizers are cleared. If finalizers are present, warn the user
			// rather than silently reporting it as deleted when it isn't gone yet.
			after, err := dr.Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
			if err == nil && after.GetDeletionTimestamp() != nil && len(after.GetFinalizers()) > 0 {
				results = append(results, kt.ResourceResult{
					Name:   obj.GetName(),
					Kind:   gvk.Kind,
					Action: kt.ActionWarning,
					Err:    fmt.Errorf("terminating — finalizers present: %v (--force to remove, v2)", after.GetFinalizers()),
				})
				continue
			}

			results = append(results, kt.ResourceResult{
				Name:   obj.GetName(),
				Kind:   gvk.Kind,
				Action: kt.ActionDeleted,
			})
		}
	}
	return results, nil
}

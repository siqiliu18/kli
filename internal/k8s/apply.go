package k8s

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// Apply deploys all Kubernetes resources found at path (file or folder) into namespace.
//
// Flow:
//  1. Collect all .yaml/.yml files from path (recursive if folder)
//  2. For each file: split on "---" to get individual YAML documents
//  3. Decode each document into an Unstructured object + extract its GVK
//  4. Use RESTMapper to translate GVK → GVR (the API server endpoint)
//  5. GET resource before apply to detect created vs configured vs unchanged
//  6. Server-side apply via dynamic client → API server creates or updates the resource
//
// Partial failures are collected and returned — apply continues even if one resource fails.
func (c *Client) Apply(path, namespace string, dryRun bool) ([]kt.ResourceResult, error) {
	files, err := collectYAMLFiles(path)
	if err != nil {
		return nil, err
	}

	// RESTMapper translates a GroupVersionKind (from the YAML) into a
	// GroupVersionResource (what the API server actually routes on).
	// e.g. apps/v1 Deployment → {Group:"apps", Version:"v1", Resource:"deployments"}
	// We fetch the full API group list from the cluster once up front so we
	// don't make a discovery call per resource.
	gr, err := restmapper.GetAPIGroupResources(c.typeClient.Discovery())
	if err != nil {
		return nil, fmt.Errorf("discover API groups: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	// NewDecodingSerializer wraps UnstructuredJSONScheme to produce a decoder
	// that can parse YAML (not just JSON). Kubernetes API objects are defined
	// in JSON schema, but config files are written in YAML — this bridge handles
	// both. UnstructuredJSONScheme tells it to decode into a generic
	// map[string]interface{} (Unstructured) rather than a typed Go struct,
	// so it works for any resource kind without needing generated types.
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	var results []kt.ResourceResult

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		for doc := range strings.SplitSeq(string(data), "\n---") {
			doc = strings.TrimSpace(doc)
			if doc == "" {
				continue
			}

			// Decode parses the YAML document into obj (populates the underlying
			// map with all fields) and extracts the GVK from apiVersion+kind fields.
			// The second return value (ignored with _) is a runtime.Object interface —
			// we already have obj for that. gvk is what we pass to RESTMapper next.
			obj := &unstructured.Unstructured{}
			_, gvk, err := decoder.Decode([]byte(doc), nil, obj)
			if err != nil {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("decode: %w", err),
				})
				continue
			}

			// Map GVK → GVR so we know which REST endpoint to call.
			// Also tells us whether the resource is namespace-scoped or cluster-scoped.
			mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("map GVK: %w", err),
				})
				continue
			}

			// Namespace-scoped: /apis/<group>/<version>/namespaces/<ns>/<resource>
			// Cluster-scoped:   /apis/<group>/<version>/<resource>
			var dr dynamic.ResourceInterface
			if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				dr = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
			} else {
				dr = c.dynamicClient.Resource(mapping.Resource)
			}

			// GET the resource before applying so we can detect the action afterwards.
			// IsNotFound → resource is new → will be ActionCreated.
			// Found → compare resourceVersion after apply → ActionConfigured or ActionUnchanged.
			existing, err := dr.Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
			preExisted := err == nil
			var preVersion string
			if preExisted {
				preVersion = existing.GetResourceVersion()
			} else if !errors.IsNotFound(err) {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("pre-check: %w", err),
				})
				continue
			}

			// Server-side apply (SSA): the API server computes the diff and
			// decides create vs. update. Force=true means kli takes ownership
			// of fields currently managed by another field manager.
			// DryRun: when set, the API server validates and diffs the object
			// but does not persist any changes — safe to use in CI previews.
			applyOpts := metav1.ApplyOptions{
				FieldManager: "kli",
				Force:        true,
			}
			if dryRun {
				applyOpts.DryRun = []string{metav1.DryRunAll}
			}
			updated, err := dr.Apply(context.TODO(), obj.GetName(), obj, applyOpts)
			if err != nil {
				results = append(results, kt.ResourceResult{
					Name: obj.GetName(), Kind: gvk.Kind, Err: fmt.Errorf("apply: %w", err),
				})
				continue
			}

			// Determine action by comparing state before and after apply.
			var action kt.Action
			switch {
			case !preExisted:
				action = kt.ActionCreated
			case updated.GetResourceVersion() != preVersion:
				action = kt.ActionConfigured
			default:
				action = kt.ActionUnchanged
			}

			results = append(results, kt.ResourceResult{
				Name:   obj.GetName(),
				Kind:   gvk.Kind,
				Action: action,
			})
		}
	}
	return results, nil
}

// collectYAMLFiles returns all .yaml/.yml files at path.
// If path is a file, it is returned directly.
// If path is a directory, it is walked recursively and all YAML files collected.
func collectYAMLFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}

	// WalkDir traverses the directory tree rooted at path in lexical order,
	// calling the callback for every file and directory it encounters —
	// including the root itself on the first call.
	// It uses DirEntry (cheaper than os.FileInfo) since it avoids an extra
	// stat syscall per entry; we only call stat when we actually need file info.
	// Returning a non-nil error from the callback stops the walk immediately.
	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

package k8s

import (
	"context"

	kt "github.com/siqiliu/kli/internal/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status fetches Deployments, StatefulSets, and DaemonSets in namespace,
// then groups their pods under each parent resource via owner references.
//
// Flow:
//  1. List Deployments, StatefulSets, DaemonSets → build []kt.ResourceStatus
//  2. Build a replicaSet → Deployment name map (pods owned by RS, not Deployment directly)
//  3. List all pods → resolve each pod's owner → attach to matching ResourceStatus
func (c *Client) Status(namespace string) ([]kt.ResourceStatus, error) {
	ctx := context.Background()
	var results []kt.ResourceStatus

	// index maps resource name → position in results slice.
	// Used later when attaching pods to their parent: instead of scanning
	// the whole results slice for each pod, we look up the position in O(1).
	index := map[string]int{}

	deployments, err := c.typeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deployments.Items {
		// Spec.Replicas is a pointer — default to 1 if unset
		total := int32(1)
		if d.Spec.Replicas != nil {
			total = *d.Spec.Replicas
		}
		index[d.Name] = len(results)
		results = append(results, kt.ResourceStatus{
			Name:   d.Name,
			Kind:   "Deployment",
			Ready:  d.Status.ReadyReplicas,
			Total:  total,
			Health: healthState(d.Status.ReadyReplicas, total),
		})
	}

	statefulsets, err := c.typeClient.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range statefulsets.Items {
		total := int32(1)
		if s.Spec.Replicas != nil {
			total = *s.Spec.Replicas
		}
		index[s.Name] = len(results)
		results = append(results, kt.ResourceStatus{
			Name:   s.Name,
			Kind:   "StatefulSet",
			Ready:  s.Status.ReadyReplicas,
			Total:  total,
			Health: healthState(s.Status.ReadyReplicas, total),
		})
	}

	daemonsets, err := c.typeClient.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, ds := range daemonsets.Items {
		// DaemonSets have no Spec.Replicas — the scheduler determines how many
		// pods to run based on node selectors. Total comes from Status instead:
		// DesiredNumberScheduled = nodes that should run this pod
		// NumberReady            = nodes currently running it and passing health checks
		index[ds.Name] = len(results)
		results = append(results, kt.ResourceStatus{
			Name:   ds.Name,
			Kind:   "DaemonSet",
			Ready:  ds.Status.NumberReady,
			Total:  ds.Status.DesiredNumberScheduled,
			Health: healthState(ds.Status.NumberReady, ds.Status.DesiredNumberScheduled),
		})
	}

	// Deployment pods have a two-level ownership chain:
	//   Pod → ReplicaSet → Deployment
	// StatefulSet and DaemonSet pods are owned directly:
	//   Pod → StatefulSet
	//   Pod → DaemonSet
	//
	// To resolve Deployment pods we need this intermediate map.
	// We list all ReplicaSets and record which Deployment each one belongs to,
	// so that when we see a pod owned by a ReplicaSet we can look up the
	// Deployment name and attach the pod to the right ResourceStatus entry.
	rsList, err := c.typeClient.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	rsToDeployment := map[string]string{} // replicaSet name → Deployment name
	for _, rs := range rsList.Items {
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" {
				rsToDeployment[rs.Name] = owner.Name
			}
		}
	}

	pods, err := c.typeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, p := range pods.Items {
		podInfo := kt.PodInfo{Name: p.Name, Phase: podPhase(p)}
		for _, owner := range p.OwnerReferences {
			parentName := owner.Name
			if owner.Kind == "ReplicaSet" {
				// Extra resolution step for Deployment pods only:
				// the pod's direct owner is a ReplicaSet, so look up which
				// Deployment owns that ReplicaSet. StatefulSet and DaemonSet
				// pods skip this branch — their owner is already the resource.
				if depName, ok := rsToDeployment[owner.Name]; ok {
					parentName = depName
				}
			}
			// Attach the pod to its parent ResourceStatus entry using the index
			// map. Pods belonging to resources not tracked (e.g. a standalone
			// ReplicaSet) are silently skipped.
			if i, ok := index[parentName]; ok {
				results[i].Pods = append(results[i].Pods, podInfo)
			}
		}
	}

	return results, nil
}

// podPhase returns the most descriptive status for a pod.
// p.Status.Phase alone is insufficient — a crash-looping container still shows
// Phase="Running". Checking ContainerStatuses.State.Waiting.Reason surfaces the
// real reason: "CrashLoopBackOff", "ImagePullBackOff", "OOMKilled", etc.
func podPhase(p corev1.Pod) string {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
	}
	return string(p.Status.Phase)
}

// healthState derives a HealthState from ready and total replica counts.
// Order of checks matters:
//   - total == 0 first: no pods scheduled yet (e.g. DaemonSet with no matching
//     nodes) — not a failure, state is simply unknown
//   - ready == 0 next: pods exist but none are ready — genuinely failed
//   - ready < total: partially healthy — degraded
//   - otherwise: all pods ready — healthy
func healthState(ready, total int32) kt.HealthState {
	if total == 0 {
		return kt.Unknown
	}
	if ready == 0 {
		return kt.Failed
	}
	if ready < total {
		return kt.Degraded
	}
	return kt.Healthy
}

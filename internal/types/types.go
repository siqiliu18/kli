package types

// HealthState represents the overall health of a workload resource.
type HealthState int

const (
	Healthy  HealthState = iota // all pods ready
	Degraded                    // some pods ready, but not all
	Failed                      // zero pods ready, total > 0
	Unknown                     // total == 0, e.g. DaemonSet with no matching nodes yet
)

// PodInfo holds display-relevant info for a single pod.
type PodInfo struct {
	Name  string
	Phase string // Running, Pending, CrashLoopBackOff, etc.
}

// ResourceStatus is returned by Status() for each workload resource
// (Deployment, StatefulSet, DaemonSet).
type ResourceStatus struct {
	Name   string
	Kind   string
	Ready  int32
	Total  int32
	Health HealthState
	Pods   []PodInfo
}

// Action represents the outcome of an apply or undeploy operation on a single resource.
type Action int

const (
	ActionCreated    Action = iota // resource did not exist, was created
	ActionConfigured               // resource existed, fields were updated
	ActionUnchanged                // resource existed, no fields changed
	ActionDeleted                  // resource was deleted
	ActionSkipped                  // resource not found during undeploy, skipped
	ActionWarning                  // resource is terminating but stuck on finalizers
)

// ResourceResult is returned by Apply() and Undeploy() for each processed resource.
type ResourceResult struct {
	Name   string
	Kind   string
	Action Action
	Err    error
}

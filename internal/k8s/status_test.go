package k8s

import (
	"testing"

	kt "github.com/siqiliu/kli/internal/types"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHealthState(t *testing.T) {
	tests := []struct {
		ready, total int32
		want         kt.HealthState
	}{
		{0, 0, kt.Unknown},
		{0, 1, kt.Failed},
		{0, 3, kt.Failed},
		{1, 3, kt.Degraded},
		{2, 3, kt.Degraded},
		{2, 2, kt.Healthy},
		{3, 3, kt.Healthy},
	}
	for _, tc := range tests {
		got := healthState(tc.ready, tc.total)
		if got != tc.want {
			t.Errorf("healthState(%d, %d) = %v, want %v", tc.ready, tc.total, got, tc.want)
		}
	}
}

func TestPodPhase(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want string
	}{
		{
			name: "running pod",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
				},
			},
			want: "Running",
		},
		{
			name: "crash loop backoff overrides Running phase",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning, // phase says Running but container is waiting
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
					},
				},
			},
			want: "CrashLoopBackOff",
		},
		{
			name: "image pull backoff",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}},
					},
				},
			},
			want: "ImagePullBackOff",
		},
		{
			name: "pending pod with no container statuses yet",
			pod: corev1.Pod{
				Status: corev1.PodStatus{Phase: corev1.PodPending},
			},
			want: "Pending",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := podPhase(tc.pod)
			if got != tc.want {
				t.Errorf("podPhase() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStatus_DeploymentWithPod(t *testing.T) {
	replicas := int32(2)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-abc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: "nginx"},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-abc-xyz",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "nginx-abc"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	c := &Client{typeClient: fake.NewSimpleClientset(deploy, rs, pod)}

	results, err := c.Status("default")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	r := results[0]
	if r.Name != "nginx" {
		t.Errorf("Name = %q, want nginx", r.Name)
	}
	if r.Kind != "Deployment" {
		t.Errorf("Kind = %q, want Deployment", r.Kind)
	}
	if r.Ready != 2 || r.Total != 2 {
		t.Errorf("Ready/Total = %d/%d, want 2/2", r.Ready, r.Total)
	}
	if r.Health != kt.Healthy {
		t.Errorf("Health = %v, want Healthy", r.Health)
	}
	if len(r.Pods) != 1 || r.Pods[0].Name != "nginx-abc-xyz" {
		t.Errorf("Pods = %v, want pod nginx-abc-xyz", r.Pods)
	}
}

func TestStatus_DegradedDeployment(t *testing.T) {
	replicas := int32(3)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	c := &Client{typeClient: fake.NewSimpleClientset(deploy)}

	results, err := c.Status("default")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if results[0].Health != kt.Degraded {
		t.Errorf("Health = %v, want Degraded", results[0].Health)
	}
}

func TestStatus_EmptyNamespace(t *testing.T) {
	c := &Client{typeClient: fake.NewSimpleClientset()}

	results, err := c.Status("empty")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNamespaceExists_Found(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "staging"}}
	c := &Client{typeClient: fake.NewSimpleClientset(ns)}

	if err := c.NamespaceExists("staging"); err != nil {
		t.Errorf("NamespaceExists(staging) = %v, want nil", err)
	}
}

func TestNamespaceExists_NotFound(t *testing.T) {
	c := &Client{typeClient: fake.NewSimpleClientset()}

	err := c.NamespaceExists("ghost")
	if err == nil {
		t.Fatal("NamespaceExists(ghost) = nil, want error")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error %q should mention the namespace name", err.Error())
	}
	if !strings.Contains(err.Error(), "kubectl create namespace") {
		t.Errorf("error %q should include kubectl hint", err.Error())
	}
}

package k8s

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestClient_Drain(t *testing.T) {
	fakeKubernetesClient := fakekubernetes.NewSimpleClientset(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	kc := NewClient(fakeKubernetesClient)
	err := kc.Drain("default", true, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

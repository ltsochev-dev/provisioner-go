package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNamespaceExistsReturnsTrueWhenNamespaceExists(t *testing.T) {
	t.Parallel()

	service, err := NewService(Config{
		Client: fake.NewSimpleClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "acme"},
		}),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	exists, err := service.NamespaceExists(context.Background(), "acme")
	if err != nil {
		t.Fatalf("namespace exists: %v", err)
	}
	if !exists {
		t.Fatal("exists = false, want true")
	}
}

func TestNamespaceExistsReturnsFalseWhenNamespaceDoesNotExist(t *testing.T) {
	t.Parallel()

	service, err := NewService(Config{Client: fake.NewSimpleClientset()})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	exists, err := service.NamespaceExists(context.Background(), "acme")
	if err != nil {
		t.Fatalf("namespace exists: %v", err)
	}
	if exists {
		t.Fatal("exists = true, want false")
	}
}

func TestCreateNamespaceCreatesNamespace(t *testing.T) {
	t.Parallel()

	service, err := NewService(Config{Client: fake.NewSimpleClientset()})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.CreateNamespace(context.Background(), "acme"); err != nil {
		t.Fatalf("create namespace: %v", err)
	}

	exists, err := service.NamespaceExists(context.Background(), "acme")
	if err != nil {
		t.Fatalf("namespace exists: %v", err)
	}
	if !exists {
		t.Fatal("exists = false, want true")
	}
}

func TestCreateNamespaceIsIdempotent(t *testing.T) {
	t.Parallel()

	service, err := NewService(Config{
		Client: fake.NewSimpleClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "acme"},
		}),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.CreateNamespace(context.Background(), "acme"); err != nil {
		t.Fatalf("create namespace: %v", err)
	}
}

func TestNamespaceNameMustBeValid(t *testing.T) {
	t.Parallel()

	service, err := NewService(Config{Client: fake.NewSimpleClientset()})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := service.NamespaceExists(context.Background(), "Not Valid"); err == nil {
		t.Fatal("namespace exists err = nil, want validation error")
	}
	if err := service.CreateNamespace(context.Background(), "Not Valid"); err == nil {
		t.Fatal("create namespace err = nil, want validation error")
	}
}

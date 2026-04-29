package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

func TestCreateOrUpdateLaravelWorkloadCreatesDeploymentAndService(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()
	service, err := NewService(Config{Client: client})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.CreateOrUpdateLaravelWorkload(context.Background(), "acme", "erp-app-acme", "example/laravel-app:latest", "laravel-env")
	if err != nil {
		t.Fatalf("create workload: %v", err)
	}

	deployment, err := client.AppsV1().Deployments("acme").Get(context.Background(), "erp-app-acme", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if deployment.Spec.Template.Spec.InitContainers[0].Name != "migrate" {
		t.Fatalf("init container = %q, want migrate", deployment.Spec.Template.Spec.InitContainers[0].Name)
	}
	if deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String() != "500m" {
		t.Fatalf("cpu request = %s, want 500m", deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
	}

	if _, err := client.CoreV1().Services("acme").Get(context.Background(), "erp-app-acme", metav1.GetOptions{}); err != nil {
		t.Fatalf("get service: %v", err)
	}
}

func TestCreateOrUpdateLaravelWorkloadUpdatesDeployment(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "erp-app-acme", Namespace: "acme"},
	})
	service, err := NewService(Config{Client: client})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.CreateOrUpdateLaravelWorkload(context.Background(), "acme", "erp-app-acme", "example/laravel-app:latest", "laravel-env")
	if err != nil {
		t.Fatalf("update workload: %v", err)
	}

	deployment, err := client.AppsV1().Deployments("acme").Get(context.Background(), "erp-app-acme", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if deployment.Spec.Template.Spec.Containers[0].Image != "example/laravel-app:latest" {
		t.Fatalf("image = %q, want placeholder image", deployment.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestScaleDeployment(t *testing.T) {
	t.Parallel()

	replicas := int32(1)
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "erp-app-acme", Namespace: "acme"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	})
	service, err := NewService(Config{Client: client})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.ScaleDeployment(context.Background(), "acme", "erp-app-acme", 0)
	if err != nil {
		t.Fatalf("scale deployment: %v", err)
	}

	deployment, err := client.AppsV1().Deployments("acme").Get(context.Background(), "erp-app-acme", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 0 {
		t.Fatalf("replicas = %v, want 0", deployment.Spec.Replicas)
	}
}

func TestScaleDeploymentIgnoresMissingDeployment(t *testing.T) {
	t.Parallel()

	service, err := NewService(Config{Client: fake.NewSimpleClientset()})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.ScaleDeployment(context.Background(), "acme", "erp-app-acme", 0); err != nil {
		t.Fatalf("scale missing deployment: %v", err)
	}
}

func TestCreateOrUpdateIngressCreatesIngress(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()
	service, err := NewService(Config{Client: client})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.CreateOrUpdateIngress(context.Background(), "acme", "erp-app-acme", "acme.example.com", "erp-app-acme")
	if err != nil {
		t.Fatalf("create ingress: %v", err)
	}

	ingress, err := client.NetworkingV1().Ingresses("acme").Get(context.Background(), "erp-app-acme", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ingress: %v", err)
	}
	if ingress.Spec.Rules[0].Host != "acme.example.com" {
		t.Fatalf("host = %q, want tenant domain", ingress.Spec.Rules[0].Host)
	}
	if ingress.Spec.IngressClassName == nil || *ingress.Spec.IngressClassName != "traefik" {
		t.Fatalf("ingress class = %v, want traefik", ingress.Spec.IngressClassName)
	}
}

func TestCreateOrUpdateIngressUpdatesIngress(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(&networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "erp-app-acme", Namespace: "acme"},
	})
	service, err := NewService(Config{Client: client})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.CreateOrUpdateIngress(context.Background(), "acme", "erp-app-acme", "acme.example.com", "erp-app-acme")
	if err != nil {
		t.Fatalf("update ingress: %v", err)
	}

	ingress, err := client.NetworkingV1().Ingresses("acme").Get(context.Background(), "erp-app-acme", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ingress: %v", err)
	}
	if ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name != "erp-app-acme" {
		t.Fatalf("service = %q, want erp-app-acme", ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
	}
}

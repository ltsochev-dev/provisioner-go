package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	RESTConfig *rest.Config
	Client     kubernetes.Interface
}

type Service struct {
	client kubernetes.Interface
}

func NewService(cfg Config) (*Service, error) {
	client := cfg.Client
	if client == nil {
		if cfg.RESTConfig == nil {
			return nil, errors.New("kubernetes rest config is required")
		}

		var err error
		client, err = kubernetes.NewForConfig(cfg.RESTConfig)
		if err != nil {
			return nil, fmt.Errorf("create kubernetes client: %w", err)
		}
	}

	return &Service{client: client}, nil
}

func RESTConfig(kubeconfigPath string) (*rest.Config, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("load kubernetes config: %w", err)
	}

	return cfg, nil
}

func (s *Service) NamespaceExists(ctx context.Context, name string) (bool, error) {
	name, err := validateNamespaceName(name)
	if err != nil {
		return false, err
	}

	_, err = s.client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get namespace %q: %w", name, err)
	}

	return true, nil
}

func (s *Service) CreateNamespace(ctx context.Context, name string) error {
	name, err := validateNamespaceName(name)
	if err != nil {
		return err
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	_, err = s.client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create namespace %q: %w", name, err)
	}

	return nil
}

func (s *Service) CreateOrUpdateSecret(ctx context.Context, ns string, name string, values map[string]string) error {
	data := make(map[string][]byte, len(values))
	for k, v := range values {
		data[k] = []byte(v)
	}

	secrets := s.client.CoreV1().Secrets(ns)

	existing, err := secrets.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = secrets.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: data,
			}, metav1.CreateOptions{})

			return err
		}

		return err
	}

	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}

	for k, v := range data {
		existing.Data[k] = v
	}

	_, err = secrets.Update(ctx, existing, metav1.UpdateOptions{})

	return err
}

func (s *Service) CreateOrUpdateLaravelWorkload(ctx context.Context, ns string, name string, image string, secretName string) error {
	labels := map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/managed-by": "provisioner",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:    "migrate",
							Image:   image,
							Command: []string{"php", "artisan", "migrate", "--force"},
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: image,
							Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 80}},
							EnvFrom: []corev1.EnvFromSource{
								{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	deployments := s.client.AppsV1().Deployments(ns)
	existingDeployment, err := deployments.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get deployment %q/%q: %w", ns, name, err)
		}
		if _, err := deployments.Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create deployment %q/%q: %w", ns, name, err)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if _, err := deployments.Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update deployment %q/%q: %w", ns, name, err)
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromString("http")},
			},
		},
	}

	services := s.client.CoreV1().Services(ns)
	existingService, err := services.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get service %q/%q: %w", ns, name, err)
		}
		if _, err := services.Create(ctx, service, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create service %q/%q: %w", ns, name, err)
		}
		return nil
	}

	service.ResourceVersion = existingService.ResourceVersion
	service.Spec.ClusterIP = existingService.Spec.ClusterIP
	service.Spec.ClusterIPs = existingService.Spec.ClusterIPs
	if _, err := services.Update(ctx, service, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update service %q/%q: %w", ns, name, err)
	}

	return nil
}

func (s *Service) CreateOrUpdateIngress(ctx context.Context, ns string, name string, host string, serviceName string) error {
	ingressClassName := "traefik"
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/name":       name,
				"app.kubernetes.io/managed-by": "provisioner",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ingresses := s.client.NetworkingV1().Ingresses(ns)
	existing, err := ingresses.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get ingress %q/%q: %w", ns, name, err)
		}
		if _, err := ingresses.Create(ctx, ingress, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create ingress %q/%q: %w", ns, name, err)
		}
		return nil
	}

	ingress.ResourceVersion = existing.ResourceVersion
	if _, err := ingresses.Update(ctx, ingress, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update ingress %q/%q: %w", ns, name, err)
	}

	return nil
}

func validateNamespaceName(name string) (string, error) {
	errs := validation.ValidateNamespaceName(name, false)
	if len(errs) > 0 {
		return "", fmt.Errorf("invalid namespace name %q: %s", name, strings.Join(errs, "; "))
	}

	return name, nil
}

func int32Ptr(value int32) *int32 {
	return &value
}

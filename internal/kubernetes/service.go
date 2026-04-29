package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func validateNamespaceName(name string) (string, error) {
	errs := validation.ValidateNamespaceName(name, false)
	if len(errs) > 0 {
		return "", fmt.Errorf("invalid namespace name %q: %s", name, strings.Join(errs, "; "))
	}

	return name, nil
}

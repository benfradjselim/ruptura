package operator

import "context"

// KubeClient abstracts Kubernetes API calls.
// The real implementation uses client-go; the stub is used in tests.
type KubeClient interface {
	DeploymentExists(ctx context.Context, namespace, name string) (bool, error)
	CreateDeployment(ctx context.Context, spec DeploymentSpec) error
	UpdateDeployment(ctx context.Context, spec DeploymentSpec) error
	DeleteDeployment(ctx context.Context, namespace, name string) error
	EnsureService(ctx context.Context, spec DeploymentSpec) error
	DeleteService(ctx context.Context, namespace, name string) error
}

// StubKubeClient is an in-memory implementation for testing.
type StubKubeClient struct {
	Deployments map[string]DeploymentSpec
	Services    map[string]DeploymentSpec
	Errors      map[string]error
}

func NewStubKubeClient() *StubKubeClient {
	return &StubKubeClient{
		Deployments: make(map[string]DeploymentSpec),
		Services:    make(map[string]DeploymentSpec),
		Errors:      make(map[string]error),
	}
}

func (s *StubKubeClient) key(ns, name string) string { return ns + "/" + name }

func (s *StubKubeClient) DeploymentExists(ctx context.Context, ns, name string) (bool, error) {
	if err := s.Errors["DeploymentExists"]; err != nil {
		return false, err
	}
	_, ok := s.Deployments[s.key(ns, name)]
	return ok, nil
}

func (s *StubKubeClient) CreateDeployment(ctx context.Context, spec DeploymentSpec) error {
	if err := s.Errors["CreateDeployment"]; err != nil {
		return err
	}
	s.Deployments[s.key(spec.Namespace, spec.Name)] = spec
	return nil
}

func (s *StubKubeClient) UpdateDeployment(ctx context.Context, spec DeploymentSpec) error {
	if err := s.Errors["UpdateDeployment"]; err != nil {
		return err
	}
	s.Deployments[s.key(spec.Namespace, spec.Name)] = spec
	return nil
}

func (s *StubKubeClient) DeleteDeployment(ctx context.Context, ns, name string) error {
	if err := s.Errors["DeleteDeployment"]; err != nil {
		return err
	}
	delete(s.Deployments, s.key(ns, name))
	return nil
}

func (s *StubKubeClient) EnsureService(ctx context.Context, spec DeploymentSpec) error {
	if err := s.Errors["EnsureService"]; err != nil {
		return err
	}
	s.Services[s.key(spec.Namespace, spec.Name)] = spec
	return nil
}

func (s *StubKubeClient) DeleteService(ctx context.Context, ns, name string) error {
	if err := s.Errors["DeleteService"]; err != nil {
		return err
	}
	delete(s.Services, s.key(ns, name))
	return nil
}

package tester_test

import (
	"testing"

	"github.com/gonest-dev/gonest/core/common"
	"github.com/gonest-dev/gonest/modules/tester"
)

type TestService struct {
	Name string
}

func (s *TestService) GetName() string {
	return s.Name
}

type TestModule struct{}

func (m *TestModule) Configure(b *common.ModuleBuilder) {
	b.Providers(&TestService{Name: "Original"})
}

func TestModule_OverrideProvider(t *testing.T) {
	compiled, err := tester.CreateModule(&TestModule{}).
		OverrideProvider((*TestService)(nil)).
		UseValue(&TestService{Name: "Mocked"}).
		Compile()

	if err != nil {
		t.Fatalf("Failed to compile testing module: %v", err)
	}

	instance, err := compiled.Get((*TestService)(nil))
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	svc := instance.(*TestService)
	if svc.GetName() != "Mocked" {
		t.Errorf("Expected 'Mocked', got '%s'", svc.GetName())
	}
}

func TestModule_UseFactory(t *testing.T) {
	compiled, err := tester.CreateModule(&TestModule{}).
		OverrideProvider((*TestService)(nil)).
		UseFactory(func() *TestService {
			return &TestService{Name: "Factory Mock"}
		}).
		Compile()

	if err != nil {
		t.Fatalf("Failed to compile testing module: %v", err)
	}

	instance, err := compiled.Get((*TestService)(nil))
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	svc := instance.(*TestService)
	if svc.GetName() != "Factory Mock" {
		t.Errorf("Expected 'Factory Mock', got '%s'", svc.GetName())
	}
}

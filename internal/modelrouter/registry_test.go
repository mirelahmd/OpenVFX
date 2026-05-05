package modelrouter

import "testing"

func TestDefaultRegistryRegistersDryRunAndStubAdapters(t *testing.T) {
	reg := NewRegistry()
	if _, ok := reg.ByName("dry-run"); !ok {
		t.Fatal("dry-run adapter not registered")
	}
	if _, ok := reg.ByName("stub"); !ok {
		t.Fatal("stub adapter not registered")
	}
}

func TestRegistryDoesNotResolveUnknownProvider(t *testing.T) {
	reg := NewRegistry()
	if _, ok := reg.ForProvider("custom-http"); ok {
		t.Fatal("unexpected adapter for custom-http")
	}
}

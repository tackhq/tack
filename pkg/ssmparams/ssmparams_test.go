package ssmparams

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockSSMAPI implements ssmGetParameterAPI for testing.
type mockSSMAPI struct {
	params map[string]string
	calls  int
}

func (m *mockSSMAPI) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	m.calls++
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	val, ok := m.params[name]
	if !ok {
		return nil, &ssmtypes.ParameterNotFound{Message: strPtr("parameter not found: " + name)}
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  input.Name,
			Value: &val,
		},
	}, nil
}

func strPtr(s string) *string { return &s }

func TestGet(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{
			"/prod/db/password": "s3cret",
			"/prod/db/host":     "db.example.com",
		},
	}
	c := newWithAPI(mock)

	val, err := c.Get(context.Background(), "/prod/db/password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "s3cret" {
		t.Errorf("expected 's3cret', got %q", val)
	}
}

func TestGetCaching(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{
			"/app/key": "value1",
		},
	}
	c := newWithAPI(mock)

	// First call
	val1, err := c.Get(context.Background(), "/app/key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call — should use cache
	val2, err := c.Get(context.Background(), "/app/key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val1 != val2 {
		t.Errorf("values differ: %q vs %q", val1, val2)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 API call, got %d", mock.calls)
	}
}

func TestGetMissing(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{},
	}
	c := newWithAPI(mock)

	_, err := c.Get(context.Background(), "/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing parameter")
	}
}

func TestGetMultipleParams(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{
			"/a": "val_a",
			"/b": "val_b",
		},
	}
	c := newWithAPI(mock)

	a, err := c.Get(context.Background(), "/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := c.Get(context.Background(), "/b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if a != "val_a" {
		t.Errorf("expected 'val_a', got %q", a)
	}
	if b != "val_b" {
		t.Errorf("expected 'val_b', got %q", b)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 API calls, got %d", mock.calls)
	}
}

func TestLazyInit(t *testing.T) {
	initCalled := false
	c := &Client{
		cache: make(map[string]string),
		initAPI: func(_ context.Context, _ string) (ssmGetParameterAPI, error) {
			initCalled = true
			return &mockSSMAPI{
				params: map[string]string{"/key": "val"},
			}, nil
		},
	}

	// Client created but no API call yet
	if initCalled {
		t.Fatal("initAPI should not be called before Get")
	}

	_, err := c.Get(context.Background(), "/key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !initCalled {
		t.Fatal("initAPI should be called on first Get")
	}
}

func TestLazyInitError(t *testing.T) {
	c := &Client{
		cache: make(map[string]string),
		initAPI: func(_ context.Context, _ string) (ssmGetParameterAPI, error) {
			return nil, fmt.Errorf("no AWS credentials")
		},
	}

	_, err := c.Get(context.Background(), "/key")
	if err == nil {
		t.Fatal("expected error when init fails")
	}
}

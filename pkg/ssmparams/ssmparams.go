// Package ssmparams provides a cached client for AWS SSM Parameter Store lookups.
package ssmparams

import (
	"context"
	"fmt"
	"sync"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ssmGetParameterAPI is the subset of the SSM client used for parameter lookups.
type ssmGetParameterAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// Client is a lazy-initialized, cached SSM Parameter Store client.
// The AWS SDK client is created on first Get() call, so playbooks that never
// use ssm_param pay zero cost. One Client per play-per-host execution.
type Client struct {
	region string
	api    ssmGetParameterAPI
	cache  map[string]string
	mu     sync.Mutex

	// initAPI allows injecting a mock for tests. When nil, a real SSM client is created.
	initAPI func(ctx context.Context, region string) (ssmGetParameterAPI, error)
}

// New creates a new lazy-init SSM Parameter Store client.
// The region parameter is optional; if empty, the AWS SDK default chain is used.
func New(region string) *Client {
	return &Client{
		region: region,
		cache:  make(map[string]string),
	}
}

// newWithAPI creates a Client with a custom API initializer (for testing).
func newWithAPI(api ssmGetParameterAPI) *Client {
	return &Client{
		api:   api,
		cache: make(map[string]string),
	}
}

// Get retrieves an SSM parameter value by name. SecureString parameters are
// automatically decrypted. Results are cached for the lifetime of the Client.
func (c *Client) Get(ctx context.Context, name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache
	if val, ok := c.cache[name]; ok {
		return val, nil
	}

	// Lazy-init the API client
	if c.api == nil {
		api, err := c.createAPI(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to create SSM client: %w", err)
		}
		c.api = api
	}

	// Fetch parameter
	out, err := c.api.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: boolPtr(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get SSM parameter %q: %w", name, err)
	}

	val := ""
	if out.Parameter != nil && out.Parameter.Value != nil {
		val = *out.Parameter.Value
	}

	c.cache[name] = val
	return val, nil
}

// createAPI initializes the real AWS SSM client.
func (c *Client) createAPI(ctx context.Context) (ssmGetParameterAPI, error) {
	if c.initAPI != nil {
		return c.initAPI(ctx, c.region)
	}

	var opts []func(*awsconfig.LoadOptions) error
	if c.region != "" {
		opts = append(opts, awsconfig.WithRegion(c.region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return ssm.NewFromConfig(cfg), nil
}

// NewWithMock creates a Client backed by a static map, for use in tests.
// The map keys are parameter names and values are the parameter values.
func NewWithMock(params map[string]string) *Client {
	return newWithAPI(&staticMock{params: params})
}

// staticMock implements ssmGetParameterAPI with a static map.
type staticMock struct {
	params map[string]string
}

func (m *staticMock) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	val, ok := m.params[name]
	if !ok {
		return nil, fmt.Errorf("parameter not found: %s", name)
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  input.Name,
			Value: &val,
		},
	}, nil
}

func boolPtr(b bool) *bool { return &b }

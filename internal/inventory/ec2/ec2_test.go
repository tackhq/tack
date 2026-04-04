package ec2

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEC2 struct {
	instances []ec2types.Instance
	err       error
}

func (m *mockEC2) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{Instances: m.instances},
		},
	}, nil
}

func makeInstance(id, privateIP, publicIP string, tags map[string]string) ec2types.Instance {
	inst := ec2types.Instance{
		InstanceId:       aws.String(id),
		PrivateIpAddress: aws.String(privateIP),
	}
	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}
	for k, v := range tags {
		inst.Tags = append(inst.Tags, ec2types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return inst
}

func TestEC2Plugin_Discovery(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "54.1.2.3", map[string]string{"Name": "web-01", "env": "prod"}),
			makeInstance("i-002", "10.0.0.2", "", map[string]string{"Name": "web-02", "env": "prod"}),
		},
	}

	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions": []any{"us-east-1"},
	})
	require.NoError(t, err)
	assert.Len(t, inv.Hosts, 2)
	assert.Contains(t, inv.Hosts, "10.0.0.1")
	assert.Contains(t, inv.Hosts, "10.0.0.2")
	assert.Equal(t, "prod", inv.Hosts["10.0.0.1"].Vars["env"])
	assert.Equal(t, "i-001", inv.Hosts["10.0.0.1"].Vars["instance_id"])
}

func TestEC2Plugin_InstanceIDHostKey(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "", map[string]string{"env": "prod"}),
		},
	}

	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions":  []any{"us-east-1"},
		"host_key": "instance_id",
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "i-001")
	// SSM connection when host_key is instance_id
	assert.Equal(t, "ssm", inv.Groups["ec2"].Connection)
}

func TestEC2Plugin_PublicIPHostKey(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "54.1.2.3", map[string]string{}),
		},
	}

	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions":  []any{"us-east-1"},
		"host_key": "public_ip",
	})
	require.NoError(t, err)
	assert.Contains(t, inv.Hosts, "54.1.2.3")
}

func TestEC2Plugin_AutoGroupByTags(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "", map[string]string{"role": "worker", "env": "prod"}),
			makeInstance("i-002", "10.0.0.2", "", map[string]string{"role": "api", "env": "prod"}),
			makeInstance("i-003", "10.0.0.3", "", map[string]string{"role": "worker", "env": "staging"}),
		},
	}

	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions":  []any{"us-east-1"},
		"group_by": []any{"tag:role", "tag:env"},
	})
	require.NoError(t, err)

	// role groups
	require.Contains(t, inv.Groups, "tag_role_worker")
	workerHosts := inv.Groups["tag_role_worker"].Hosts
	sort.Strings(workerHosts)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.3"}, workerHosts)

	require.Contains(t, inv.Groups, "tag_role_api")
	assert.Equal(t, []string{"10.0.0.2"}, inv.Groups["tag_role_api"].Hosts)

	// env groups
	require.Contains(t, inv.Groups, "tag_env_prod")
	prodHosts := inv.Groups["tag_env_prod"].Hosts
	sort.Strings(prodHosts)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, prodHosts)

	require.Contains(t, inv.Groups, "tag_env_staging")
	assert.Equal(t, []string{"10.0.0.3"}, inv.Groups["tag_env_staging"].Hosts)
}

func TestEC2Plugin_InstanceMissingGroupByTag(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "", map[string]string{"role": "worker"}),
			makeInstance("i-002", "10.0.0.2", "", map[string]string{}), // no role tag
		},
	}

	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions":  []any{"us-east-1"},
		"group_by": []any{"tag:role"},
	})
	require.NoError(t, err)

	// Both hosts in inventory
	assert.Len(t, inv.Hosts, 2)
	// Only i-001 in the role group
	require.Contains(t, inv.Groups, "tag_role_worker")
	assert.Equal(t, []string{"10.0.0.1"}, inv.Groups["tag_role_worker"].Hosts)
}

func TestEC2Plugin_EmptyResult(t *testing.T) {
	mock := &mockEC2{instances: nil}
	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions": []any{"us-east-1"},
	})
	require.NoError(t, err)
	assert.Empty(t, inv.Hosts)
}

func TestEC2Plugin_APIError(t *testing.T) {
	mock := &mockEC2{err: fmt.Errorf("AccessDenied")}
	p := &Plugin{client: mock}
	_, err := p.Load(context.Background(), map[string]any{
		"regions": []any{"us-east-1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AccessDenied")
}

func TestEC2Plugin_MissingRegions(t *testing.T) {
	p := &Plugin{}
	_, err := p.Load(context.Background(), map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regions")
}

func TestEC2Plugin_TagsAsHostVars(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "", map[string]string{"Name": "web-01", "my-tag": "my-value"}),
		},
	}

	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions": []any{"us-east-1"},
	})
	require.NoError(t, err)

	vars := inv.Hosts["10.0.0.1"].Vars
	assert.Equal(t, "web-01", vars["name"])
	assert.Equal(t, "my-value", vars["my_tag"]) // hyphens → underscores
}

func TestEC2Plugin_ConnectionDefaults(t *testing.T) {
	mock := &mockEC2{
		instances: []ec2types.Instance{
			makeInstance("i-001", "10.0.0.1", "", map[string]string{}),
		},
	}

	// private_ip → ssh
	p := &Plugin{client: mock}
	inv, err := p.Load(context.Background(), map[string]any{
		"regions": []any{"us-east-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ssh", inv.Groups["ec2"].Connection)

	// instance_id → ssm
	inv, err = p.Load(context.Background(), map[string]any{
		"regions":  []any{"us-east-1"},
		"host_key": "instance_id",
	})
	require.NoError(t, err)
	assert.Equal(t, "ssm", inv.Groups["ec2"].Connection)
}

func TestNormalizeTagKey(t *testing.T) {
	assert.Equal(t, "name", normalizeTagKey("Name"))
	assert.Equal(t, "my_tag", normalizeTagKey("my-tag"))
	assert.Equal(t, "my_long_tag", normalizeTagKey("My-Long-Tag"))
}

func TestSanitizeGroupName(t *testing.T) {
	assert.Equal(t, "tag_role_worker", sanitizeGroupName("tag_role_worker"))
	assert.Equal(t, "tag_env_us_east_1", sanitizeGroupName("tag_env_us-east-1"))
}

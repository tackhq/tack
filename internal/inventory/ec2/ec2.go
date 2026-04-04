// Package ec2 implements an inventory plugin that discovers hosts from
// AWS EC2 instances using DescribeInstances with tag-based filtering.
package ec2

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/eugenetaranov/bolt/internal/inventory"
)

func init() {
	inventory.RegisterPlugin(&Plugin{})
}

// ec2API is the subset of the EC2 client used for instance discovery.
type ec2API interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// Plugin implements the inventory.Plugin interface for EC2-based inventory.
type Plugin struct {
	// client is an optional override for testing.
	client ec2API
}

func (p *Plugin) Name() string { return "ec2" }

func (p *Plugin) Load(ctx context.Context, config map[string]any) (*inventory.Inventory, error) {
	regions, err := parseStringSlice(config, "regions")
	if err != nil || len(regions) == 0 {
		return nil, fmt.Errorf("ec2 plugin: 'regions' is required (list of AWS region strings)")
	}

	filters := parseStringMap(config, "filters")
	groupBy, _ := parseStringSlice(config, "group_by")
	hostKey := "private_ip"
	if hk, ok := config["host_key"].(string); ok && hk != "" {
		hostKey = hk
	}

	inv := &inventory.Inventory{
		Hosts:  make(map[string]*inventory.HostEntry),
		Groups: make(map[string]*inventory.GroupEntry),
	}

	for _, region := range regions {
		client := p.client
		if client == nil {
			cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
			if err != nil {
				return nil, fmt.Errorf("ec2 plugin: failed to load AWS config for %s: %w", region, err)
			}
			client = ec2.NewFromConfig(cfg)
		}

		instances, err := describeInstances(ctx, client, filters)
		if err != nil {
			return nil, fmt.Errorf("ec2 plugin: %s: %w", region, err)
		}

		for _, inst := range instances {
			hostName := instanceHostKey(inst, hostKey)
			if hostName == "" {
				continue
			}

			// Build host vars from tags
			vars := make(map[string]any)
			for _, tag := range inst.Tags {
				if tag.Key != nil && tag.Value != nil {
					key := normalizeTagKey(*tag.Key)
					vars[key] = *tag.Value
				}
			}
			vars["instance_id"] = *inst.InstanceId
			vars["region"] = region
			if inst.PrivateIpAddress != nil {
				vars["private_ip"] = *inst.PrivateIpAddress
			}
			if inst.PublicIpAddress != nil {
				vars["public_ip"] = *inst.PublicIpAddress
			}

			inv.Hosts[hostName] = &inventory.HostEntry{Vars: vars}

			// Auto-group by tags
			for _, gKey := range groupBy {
				tagKey := strings.TrimPrefix(gKey, "tag:")
				for _, tag := range inst.Tags {
					if tag.Key != nil && *tag.Key == tagKey && tag.Value != nil {
						groupName := sanitizeGroupName(fmt.Sprintf("tag_%s_%s", tagKey, *tag.Value))
						g, ok := inv.Groups[groupName]
						if !ok {
							conn := "ssh"
							if hostKey == "instance_id" {
								conn = "ssm"
							}
							g = &inventory.GroupEntry{
								Connection: conn,
								Hosts:      []string{},
							}
							inv.Groups[groupName] = g
						}
						// Deduplicate
						found := false
						for _, h := range g.Hosts {
							if h == hostName {
								found = true
								break
							}
						}
						if !found {
							g.Hosts = append(g.Hosts, hostName)
						}
					}
				}
			}
		}
	}

	// If no group_by but we have hosts, create an "all" group with connection default
	if len(groupBy) == 0 && len(inv.Hosts) > 0 {
		conn := "ssh"
		if hostKey == "instance_id" {
			conn = "ssm"
		}
		allGroup := &inventory.GroupEntry{
			Connection: conn,
			Hosts:      make([]string, 0, len(inv.Hosts)),
		}
		for name := range inv.Hosts {
			allGroup.Hosts = append(allGroup.Hosts, name)
		}
		inv.Groups["ec2"] = allGroup
	}

	return inv, nil
}

func describeInstances(ctx context.Context, client ec2API, filters map[string]string) ([]ec2types.Instance, error) {
	ec2Filters := []ec2types.Filter{
		{
			Name:   aws.String("instance-state-name"),
			Values: []string{"running"},
		},
	}
	for k, v := range filters {
		ec2Filters = append(ec2Filters, ec2types.Filter{
			Name:   aws.String(k),
			Values: []string{v},
		})
	}

	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: ec2Filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []ec2types.Instance
	for _, res := range out.Reservations {
		instances = append(instances, res.Instances...)
	}
	return instances, nil
}

func instanceHostKey(inst ec2types.Instance, hostKey string) string {
	switch hostKey {
	case "instance_id":
		if inst.InstanceId != nil {
			return *inst.InstanceId
		}
	case "public_ip":
		if inst.PublicIpAddress != nil {
			return *inst.PublicIpAddress
		}
	default: // private_ip
		if inst.PrivateIpAddress != nil {
			return *inst.PrivateIpAddress
		}
	}
	return ""
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func normalizeTagKey(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "-", "_"))
}

func sanitizeGroupName(name string) string {
	return strings.Trim(nonAlphaNum.ReplaceAllString(strings.ToLower(name), "_"), "_")
}

func parseStringSlice(config map[string]any, key string) ([]string, error) {
	raw, ok := config[key]
	if !ok {
		return nil, nil
	}
	switch v := raw.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("expected list for %q, got %T", key, raw)
	}
}

func parseStringMap(config map[string]any, key string) map[string]string {
	raw, ok := config[key].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

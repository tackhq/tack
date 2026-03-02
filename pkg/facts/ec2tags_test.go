package facts

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type mockEC2Client struct {
	tags []types.TagDescription
	err  error
}

func (m *mockEC2Client) DescribeTags(_ context.Context, _ *ec2.DescribeTagsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ec2.DescribeTagsOutput{Tags: m.tags}, nil
}

func TestGatherEC2Tags_Success(t *testing.T) {
	orig := newEC2Client
	defer func() { newEC2Client = orig }()

	newEC2Client = func(_ context.Context, region string) (ec2DescribeTagsAPI, error) {
		if region != "us-east-1" {
			t.Errorf("expected region us-east-1, got %s", region)
		}
		return &mockEC2Client{
			tags: []types.TagDescription{
				{Key: aws.String("Name"), Value: aws.String("web-1")},
				{Key: aws.String("Env"), Value: aws.String("prod")},
			},
		}, nil
	}

	tags, err := gatherEC2Tags(context.Background(), "i-abc123", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tags["Name"] != "web-1" {
		t.Errorf("tags[Name] = %q, want web-1", tags["Name"])
	}
	if tags["Env"] != "prod" {
		t.Errorf("tags[Env] = %q, want prod", tags["Env"])
	}
}

func TestGatherEC2Tags_APIError(t *testing.T) {
	orig := newEC2Client
	defer func() { newEC2Client = orig }()

	newEC2Client = func(_ context.Context, _ string) (ec2DescribeTagsAPI, error) {
		return &mockEC2Client{err: fmt.Errorf("access denied")}, nil
	}

	tags, err := gatherEC2Tags(context.Background(), "i-abc123", "us-east-1")
	if err != nil {
		t.Fatalf("expected nil error for best-effort, got %v", err)
	}
	if tags != nil {
		t.Errorf("expected nil tags on error, got %v", tags)
	}
}

func TestGatherEC2Tags_ClientError(t *testing.T) {
	orig := newEC2Client
	defer func() { newEC2Client = orig }()

	newEC2Client = func(_ context.Context, _ string) (ec2DescribeTagsAPI, error) {
		return nil, fmt.Errorf("config error")
	}

	tags, err := gatherEC2Tags(context.Background(), "i-abc123", "us-east-1")
	if err != nil {
		t.Fatalf("expected nil error for best-effort, got %v", err)
	}
	if tags != nil {
		t.Errorf("expected nil tags on client error, got %v", tags)
	}
}

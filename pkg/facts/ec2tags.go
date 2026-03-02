package facts

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ec2DescribeTagsAPI is the subset of the EC2 client needed for tag lookup.
// Defined as an interface to allow testing with a mock.
type ec2DescribeTagsAPI interface {
	DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error)
}

// newEC2Client creates a real EC2 client for the given region.
// It is a package-level variable so tests can replace it.
var newEC2Client = func(ctx context.Context, region string) (ec2DescribeTagsAPI, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return ec2.NewFromConfig(cfg), nil
}

// gatherEC2Tags fetches instance tags via the EC2 DescribeTags API.
// Returns nil, nil if the call fails (best-effort).
func gatherEC2Tags(ctx context.Context, instanceID, region string) (map[string]string, error) {
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return nil, nil
	}

	out, err := client.DescribeTags(ctx, &ec2.DescribeTagsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: []string{instanceID},
			},
			{
				Name:   aws.String("resource-type"),
				Values: []string{"instance"},
			},
		},
	})
	if err != nil {
		return nil, nil
	}

	tags := make(map[string]string, len(out.Tags))
	for _, t := range out.Tags {
		if t.Key != nil && t.Value != nil {
			tags[*t.Key] = *t.Value
		}
	}
	return tags, nil
}

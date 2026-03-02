// Package ssm provides a connector for executing commands on EC2 instances via AWS Systems Manager.
package ssm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/eugenetaranov/bolt/internal/connector"
)

// Default settings.
const (
	defaultTimeout  = 10 * time.Minute
	pollInterval    = 2 * time.Second
	maxBase64Bytes  = 24 * 1024 // 24 KB limit for base64 inline transfer
	s3KeyPrefix     = "bolt-transfer/"
)

// ssmAPI is the subset of the SSM client used by the connector.
type ssmAPI interface {
	DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
	CancelCommand(ctx context.Context, params *ssm.CancelCommandInput, optFns ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error)
}

// s3API is the subset of the S3 client used by the connector.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// ec2API is the subset of the EC2 client used for tag-based instance resolution.
type ec2API interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// Connector executes commands on EC2 instances via AWS Systems Manager.
type Connector struct {
	instanceID   string
	region       string
	bucket       string // S3 bucket for file transfer; empty = base64 fallback
	timeout      time.Duration
	sudo         bool
	sudoPassword string
	ssmClient    ssmAPI
	s3Client     s3API
}

// Option configures the SSM connector.
type Option func(*Connector)

// WithRegion sets the AWS region.
func WithRegion(region string) Option {
	return func(c *Connector) {
		c.region = region
	}
}

// WithBucket sets the S3 bucket for file transfers.
func WithBucket(bucket string) Option {
	return func(c *Connector) {
		c.bucket = bucket
	}
}

// WithTimeout sets the command execution timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Connector) {
		c.timeout = d
	}
}

// WithSudo enables sudo for command execution.
func WithSudo() Option {
	return func(c *Connector) {
		c.sudo = true
	}
}

// WithSudoPassword sets the sudo password.
func WithSudoPassword(password string) Option {
	return func(c *Connector) {
		c.sudoPassword = password
	}
}

// withSSMClient injects a custom SSM client (for testing).
func withSSMClient(client ssmAPI) Option {
	return func(c *Connector) {
		c.ssmClient = client
	}
}

// withS3Client injects a custom S3 client (for testing).
func withS3Client(client s3API) Option {
	return func(c *Connector) {
		c.s3Client = client
	}
}

// New creates a new SSM connector for the specified instance ID.
func New(instanceID string, opts ...Option) *Connector {
	c := &Connector{
		instanceID: instanceID,
		timeout:    defaultTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Connect validates the instance is SSM-managed. If no SSM client was injected,
// it loads the AWS config and creates real SSM (and optionally S3) clients.
func (c *Connector) Connect(ctx context.Context) error {
	if c.ssmClient == nil {
		var optFns []func(*awsconfig.LoadOptions) error
		if c.region != "" {
			optFns = append(optFns, awsconfig.WithRegion(c.region))
		}
		cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w", err)
		}
		c.ssmClient = ssm.NewFromConfig(cfg)
		if c.bucket != "" {
			c.s3Client = s3.NewFromConfig(cfg)
		}
	}

	// Validate instance is SSM-managed
	out, err := c.ssmClient.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    aws.String("InstanceIds"),
				Values: []string{c.instanceID},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe instance %s: %w", c.instanceID, err)
	}
	if len(out.InstanceInformationList) == 0 {
		return fmt.Errorf("instance %s is not managed by SSM (check SSM agent and IAM role)", c.instanceID)
	}

	return nil
}

// Execute runs a command on the instance via SSM SendCommand and polls for the result.
func (c *Connector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	fullCmd := c.buildCommand(cmd)

	timeoutSec := int(c.timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 600
	}

	sendOut, err := c.ssmClient.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  []string{c.instanceID},
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			"commands":         {fullCmd},
			"executionTimeout": {fmt.Sprintf("%d", timeoutSec)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send command to %s: %w", c.instanceID, err)
	}

	commandID := aws.ToString(sendOut.Command.CommandId)

	// Poll for completion
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Best-effort cancel with a fresh context
			cancelCtx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelFn()
			_, _ = c.ssmClient.CancelCommand(cancelCtx, &ssm.CancelCommandInput{
				CommandId: aws.String(commandID),
			})
			return nil, ctx.Err()
		case <-ticker.C:
			invOut, err := c.ssmClient.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(c.instanceID),
			})
			if err != nil {
				// InvocationDoesNotExist is transient — command may not have registered yet
				if strings.Contains(err.Error(), "InvocationDoesNotExist") {
					continue
				}
				return nil, fmt.Errorf("failed to get command invocation: %w", err)
			}

			switch invOut.Status {
			case ssmtypes.CommandInvocationStatusPending,
				ssmtypes.CommandInvocationStatusInProgress,
				ssmtypes.CommandInvocationStatusDelayed:
				continue

			case ssmtypes.CommandInvocationStatusSuccess:
				return &connector.Result{
					Stdout:   aws.ToString(invOut.StandardOutputContent),
					Stderr:   aws.ToString(invOut.StandardErrorContent),
					ExitCode: 0,
				}, nil

			case ssmtypes.CommandInvocationStatusFailed:
				exitCode := int(invOut.ResponseCode)
				return &connector.Result{
					Stdout:   aws.ToString(invOut.StandardOutputContent),
					Stderr:   aws.ToString(invOut.StandardErrorContent),
					ExitCode: exitCode,
				}, nil

			case ssmtypes.CommandInvocationStatusTimedOut:
				return nil, fmt.Errorf("command timed out on %s", c.instanceID)

			case ssmtypes.CommandInvocationStatusCancelled:
				return nil, fmt.Errorf("command was cancelled on %s", c.instanceID)

			default:
				return nil, fmt.Errorf("unexpected command status %s on %s", invOut.Status, c.instanceID)
			}
		}
	}
}

// Upload copies content to the remote instance.
// Uses S3 as a transfer medium if a bucket is configured, otherwise falls back to base64 inline.
func (c *Connector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("failed to read upload source: %w", err)
	}

	modeStr := fmt.Sprintf("%04o", mode)

	if c.bucket != "" {
		return c.uploadViaS3(ctx, data, dst, modeStr)
	}

	return c.uploadViaBase64(ctx, data, dst, modeStr)
}

// uploadViaS3 uploads data through an S3 bucket.
func (c *Connector) uploadViaS3(ctx context.Context, data []byte, dst, modeStr string) error {
	key := s3KeyPrefix + c.instanceID + "/" + fmt.Sprintf("%d", time.Now().UnixNano())

	_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Copy from S3 to destination on instance
	cmd := fmt.Sprintf("aws s3 cp s3://%s/%s %s && chmod %s %s",
		connector.ShellQuote(c.bucket), connector.ShellQuote(key), connector.ShellQuote(dst), modeStr, connector.ShellQuote(dst))
	result, err := c.Execute(ctx, cmd)
	if err != nil {
		c.cleanupS3(ctx, key)
		return fmt.Errorf("failed to copy from S3 to %s: %w", dst, err)
	}
	if result.ExitCode != 0 {
		c.cleanupS3(ctx, key)
		return fmt.Errorf("failed to copy from S3 to %s: %s", dst, result.Stderr)
	}

	c.cleanupS3(ctx, key)
	return nil
}

// uploadViaBase64 uploads data inline using base64 encoding.
func (c *Connector) uploadViaBase64(ctx context.Context, data []byte, dst, modeStr string) error {
	if len(data) > maxBase64Bytes {
		return fmt.Errorf("file too large (%d bytes) for inline transfer; use --ssm-bucket for files over %d bytes",
			len(data), maxBase64Bytes)
	}

	b64 := base64.StdEncoding.EncodeToString(data)

	// Ensure parent directory exists
	cmd := fmt.Sprintf("mkdir -p %s && printf '%%s' '%s' | base64 -d > %s && chmod %s %s",
		connector.ShellQuote(dirOf(dst)), b64, connector.ShellQuote(dst), modeStr, connector.ShellQuote(dst))
	result, err := c.Execute(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to upload to %s: %w", dst, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to upload to %s: %s", dst, result.Stderr)
	}

	return nil
}

// Download copies content from the remote instance.
func (c *Connector) Download(ctx context.Context, src string, dst io.Writer) error {
	if c.bucket != "" {
		return c.downloadViaS3(ctx, src, dst)
	}

	return c.downloadViaBase64(ctx, src, dst)
}

// downloadViaS3 downloads data through an S3 bucket.
func (c *Connector) downloadViaS3(ctx context.Context, src string, dst io.Writer) error {
	key := s3KeyPrefix + c.instanceID + "/" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Copy from instance to S3
	cmd := fmt.Sprintf("aws s3 cp %s s3://%s/%s", connector.ShellQuote(src), connector.ShellQuote(c.bucket), connector.ShellQuote(key))
	result, err := c.Execute(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to copy %s to S3: %w", src, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to copy %s to S3: %s", src, result.Stderr)
	}

	// Get from S3
	getOut, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.cleanupS3(ctx, key)
		return fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer getOut.Body.Close()

	if _, err := io.Copy(dst, getOut.Body); err != nil {
		c.cleanupS3(ctx, key)
		return fmt.Errorf("failed to read S3 object: %w", err)
	}

	c.cleanupS3(ctx, key)
	return nil
}

// downloadViaBase64 downloads data inline using base64 encoding.
func (c *Connector) downloadViaBase64(ctx context.Context, src string, dst io.Writer) error {
	result, err := c.Execute(ctx, fmt.Sprintf("base64 %s", connector.ShellQuote(src)))
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to read %s: %s", src, result.Stderr)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Stdout))
	if err != nil {
		return fmt.Errorf("failed to decode base64 output: %w", err)
	}

	if _, err := dst.Write(decoded); err != nil {
		return fmt.Errorf("failed to write downloaded content: %w", err)
	}

	return nil
}

// SetSudo enables or disables sudo for subsequent commands.
func (c *Connector) SetSudo(enabled bool, password string) {
	c.sudo = enabled
	c.sudoPassword = password
}

// Close is a no-op for SSM (no persistent connection).
func (c *Connector) Close() error {
	return nil
}

// String returns a human-readable description of the connection.
func (c *Connector) String() string {
	desc := fmt.Sprintf("ssm://%s", c.instanceID)
	if c.region != "" {
		desc += fmt.Sprintf(" (region=%s)", c.region)
	}
	if c.sudo {
		desc += " (sudo)"
	}
	return desc
}

// buildCommand wraps the command with sudo if configured.
func (c *Connector) buildCommand(cmd string) string {
	return connector.BuildSudoCommand(cmd, c.sudo, c.sudoPassword, false)
}

// cleanupS3 removes a temporary S3 object (best-effort).
func (c *Connector) cleanupS3(ctx context.Context, key string) {
	if c.s3Client == nil {
		return
	}
	_, _ = c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
}


// dirOf returns the directory component of a path.
func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	return path[:i]
}

// ResolveInstancesByTags uses EC2 DescribeInstances to find running instances
// matching the given tags. Returns a list of instance IDs.
func ResolveInstancesByTags(ctx context.Context, tags map[string]string, region string) ([]string, error) {
	var optFns []func(*awsconfig.LoadOptions) error
	if region != "" {
		optFns = append(optFns, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return resolveInstancesByTagsWithClient(ctx, ec2.NewFromConfig(cfg), tags)
}

// resolveInstancesByTagsWithClient is the testable core of ResolveInstancesByTags.
func resolveInstancesByTagsWithClient(ctx context.Context, client ec2API, tags map[string]string) ([]string, error) {
	filters := []ec2types.Filter{
		{
			Name:   aws.String("instance-state-name"),
			Values: []string{"running"},
		},
	}
	for k, v := range tags {
		filters = append(filters, ec2types.Filter{
			Name:   aws.String("tag:" + k),
			Values: []string{v},
		})
	}

	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var ids []string
	for _, res := range out.Reservations {
		for _, inst := range res.Instances {
			if inst.InstanceId != nil {
				ids = append(ids, *inst.InstanceId)
			}
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no running instances found matching tags: %v", tags)
	}

	return ids, nil
}

// Ensure Connector implements the connector.Connector interface.
var _ connector.Connector = (*Connector)(nil)

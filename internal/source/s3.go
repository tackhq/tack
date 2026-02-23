package source

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// S3Source downloads a playbook from an S3 bucket using the AWS CLI.
type S3Source struct {
	Bucket string
	Key    string
}

func parseS3Source(ref string) (*S3Source, error) {
	// ref is "s3://bucket/key/to/playbook.yaml"
	without := strings.TrimPrefix(ref, "s3://")
	idx := strings.Index(without, "/")
	if idx == -1 {
		return nil, fmt.Errorf("invalid S3 path (missing key): %s", ref)
	}
	return &S3Source{
		Bucket: without[:idx],
		Key:    without[idx+1:],
	}, nil
}

func (s *S3Source) Fetch(ctx context.Context) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "bolt-s3-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	// Download the parent directory recursively so roles/includes work
	s3Dir := fmt.Sprintf("s3://%s/%s", s.Bucket, filepath.Dir(s.Key))
	cmd := exec.CommandContext(ctx, "aws", "s3", "cp", "--recursive", s3Dir, tmpDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("aws s3 cp failed: %w", err)
	}

	playbookPath := filepath.Join(tmpDir, filepath.Base(s.Key))
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		cleanup()
		return "", nil, fmt.Errorf("playbook not found after download: %s", filepath.Base(s.Key))
	}

	return playbookPath, cleanup, nil
}

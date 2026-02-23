package source

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitSource clones a git repository and extracts a playbook from it.
type GitSource struct {
	RepoURL string
	Ref     string // branch, tag, or commit (empty = default branch)
	Path    string // path within the repo
}

func parseGitSource(ref string) (*GitSource, error) {
	repo, path, err := splitRepoPath(ref)
	if err != nil {
		return nil, err
	}
	repoURL, gitRef := splitRepoRef(repo)
	return &GitSource{
		RepoURL: repoURL,
		Ref:     gitRef,
		Path:    path,
	}, nil
}

func (s *GitSource) Fetch(ctx context.Context) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "bolt-git-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	args := []string{"clone", "--depth", "1"}
	if s.Ref != "" {
		args = append(args, "--branch", s.Ref)
	}
	args = append(args, s.RepoURL, tmpDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git clone failed: %w", err)
	}

	playbookPath := filepath.Join(tmpDir, s.Path)
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		cleanup()
		return "", nil, fmt.Errorf("playbook not found in repo: %s", s.Path)
	}

	return playbookPath, cleanup, nil
}

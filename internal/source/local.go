package source

import (
	"context"
	"fmt"
	"os"
)

// LocalSource wraps a local file path.
type LocalSource struct {
	Path string
}

func (s *LocalSource) Fetch(_ context.Context) (string, func(), error) {
	if _, err := os.Stat(s.Path); err != nil {
		return "", nil, fmt.Errorf("playbook not found: %s: %w", s.Path, err)
	}
	return s.Path, noop, nil
}

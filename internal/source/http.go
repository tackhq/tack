package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// HTTPSource downloads a single playbook file over HTTP(S).
type HTTPSource struct {
	URL string
}

func parseHTTPSource(ref string) (*HTTPSource, error) {
	return &HTTPSource{URL: ref}, nil
}

func (s *HTTPSource) Fetch(ctx context.Context) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "tack-http-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("downloading playbook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Use the last path segment as filename, fallback to "playbook.yaml"
	filename := filepath.Base(s.URL)
	if filename == "" || filename == "." || filename == "/" {
		filename = "playbook.yaml"
	}

	playbookPath := filepath.Join(tmpDir, filename)
	f, err := os.Create(playbookPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("writing playbook: %w", err)
	}

	return playbookPath, cleanup, nil
}

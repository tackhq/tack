package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_Local(t *testing.T) {
	src, err := Resolve("./playbooks/setup.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.(*LocalSource); !ok {
		t.Fatalf("expected LocalSource, got %T", src)
	}
}

func TestResolve_GitSSH(t *testing.T) {
	src, err := Resolve("git@github.com:user/repo.git//path/to/playbook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gs, ok := src.(*GitSource)
	if !ok {
		t.Fatalf("expected GitSource, got %T", src)
	}
	if gs.RepoURL != "git@github.com:user/repo.git" {
		t.Errorf("RepoURL = %q", gs.RepoURL)
	}
	if gs.Ref != "" {
		t.Errorf("Ref = %q, want empty", gs.Ref)
	}
	if gs.Path != "path/to/playbook.yaml" {
		t.Errorf("Path = %q", gs.Path)
	}
}

func TestResolve_GitSSHWithRef(t *testing.T) {
	src, err := Resolve("git@github.com:user/repo.git@main//path/to/playbook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gs := src.(*GitSource)
	if gs.RepoURL != "git@github.com:user/repo.git" {
		t.Errorf("RepoURL = %q", gs.RepoURL)
	}
	if gs.Ref != "main" {
		t.Errorf("Ref = %q, want main", gs.Ref)
	}
	if gs.Path != "path/to/playbook.yaml" {
		t.Errorf("Path = %q", gs.Path)
	}
}

func TestResolve_GitHTTPS(t *testing.T) {
	src, err := Resolve("https://github.com/user/repo.git//playbook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gs := src.(*GitSource)
	if gs.RepoURL != "https://github.com/user/repo.git" {
		t.Errorf("RepoURL = %q", gs.RepoURL)
	}
	if gs.Ref != "" {
		t.Errorf("Ref = %q, want empty", gs.Ref)
	}
}

func TestResolve_GitHTTPSWithRef(t *testing.T) {
	src, err := Resolve("https://github.com/user/repo.git@v1.0//playbook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gs := src.(*GitSource)
	if gs.RepoURL != "https://github.com/user/repo.git" {
		t.Errorf("RepoURL = %q", gs.RepoURL)
	}
	if gs.Ref != "v1.0" {
		t.Errorf("Ref = %q, want v1.0", gs.Ref)
	}
}

func TestResolve_S3(t *testing.T) {
	src, err := Resolve("s3://bucket/path/to/playbook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ss, ok := src.(*S3Source)
	if !ok {
		t.Fatalf("expected S3Source, got %T", src)
	}
	if ss.Bucket != "bucket" {
		t.Errorf("Bucket = %q", ss.Bucket)
	}
	if ss.Key != "path/to/playbook.yaml" {
		t.Errorf("Key = %q", ss.Key)
	}
}

func TestResolve_HTTP(t *testing.T) {
	src, err := Resolve("https://example.com/playbook.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.(*HTTPSource); !ok {
		t.Fatalf("expected HTTPSource, got %T", src)
	}
}

func TestLocalSource_Fetch(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	pbPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(pbPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	src := &LocalSource{Path: pbPath}
	path, cleanup, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if path != pbPath {
		t.Errorf("got %q, want %q", path, pbPath)
	}
}

func TestLocalSource_Fetch_NotFound(t *testing.T) {
	src := &LocalSource{Path: "/nonexistent/playbook.yaml"}
	_, _, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestResolve_GitSSH_MissingSeparator(t *testing.T) {
	_, err := Resolve("git@github.com:user/repo.git")
	if err == nil {
		t.Fatal("expected error for missing // separator")
	}
}

func TestResolve_S3_MissingKey(t *testing.T) {
	_, err := Resolve("s3://bucket")
	if err == nil {
		t.Fatal("expected error for missing S3 key")
	}
}

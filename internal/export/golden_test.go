package export

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/tackhq/tack/internal/playbook"

	// Register modules used by the golden playbooks.
	_ "github.com/tackhq/tack/internal/module/cron"
	_ "github.com/tackhq/tack/internal/module/git"
	_ "github.com/tackhq/tack/internal/module/group"
	_ "github.com/tackhq/tack/internal/module/lineinfile"
	_ "github.com/tackhq/tack/internal/module/systemd"
	_ "github.com/tackhq/tack/internal/module/user"
)

// update regenerates golden files: `go test ./internal/export -run TestGolden -update`.
var update = flag.Bool("update", false, "update golden files")

// goldenCases maps a playbook fixture to the host it is compiled for.
var goldenCases = []struct {
	file string
	host string
}{
	{"web-server.yaml", "web01"},
	{"users-and-jobs.yaml", "db01"},
	{"mixed-support.yaml", "app01"},
}

// TestGolden compiles representative full playbooks and compares the emitted
// script against a checked-in golden file (15.2). Run with -update to refresh.
func TestGolden(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("testdata", "golden", tc.file)
			pb, err := playbook.ParseFileRaw(path)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if len(pb.Plays) == 0 {
				t.Fatalf("%s has no plays", path)
			}

			c := &Compiler{
				Playbook: pb,
				Opts: Options{
					Version:           "test",
					PlaybookPath:      tc.file,
					NoFacts:           true,
					NoBannerTimestamp: true,
				},
				PlaybookDir: filepath.Dir(path),
			}

			res, err := c.Compile(context.Background(), pb.Plays[0], tc.host, nil)
			if err != nil {
				t.Fatalf("compile %s: %v", path, err)
			}

			goldenPath := filepath.Join("testdata", "golden", tc.file+".golden")
			if *update {
				if err := os.WriteFile(goldenPath, []byte(res.Script), 0644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if res.Script != string(want) {
				t.Errorf("script mismatch for %s (run with -update to refresh)\n--- got ---\n%s", tc.file, res.Script)
			}
		})
	}
}

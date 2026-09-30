package connector

import (
	"context"
	"testing"
)

func TestSummarizeCommand(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"apt-get install -y nginx", 80, "apt-get install -y nginx"},
		{"\n\n  set -e\n  echo hi\n", 80, "set -e"},
		{"  echo   a\t\tb ", 80, "echo a b"},
		{"0123456789abcdef", 10, "012345678…"},
	}
	for _, tt := range tests {
		if got := SummarizeCommand(tt.in, tt.max); got != tt.want {
			t.Errorf("SummarizeCommand(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestProgress_SuppressCommands(t *testing.T) {
	var got []string
	ctx := WithProgress(context.Background(), func(_ ProgressKind, msg string) { got = append(got, msg) })
	quiet := SuppressCommands(ctx)
	ReportCommand(quiet, "cat secret")
	ReportProgress(quiet, "uploading %s", "/x")
	if len(got) != 1 || got[0] != "uploading /x" {
		t.Errorf("got %v, want only the status update", got)
	}
	// No sink: reporting is a no-op.
	ReportCommand(context.Background(), "x")
}

package cron

import (
	"strings"
	"testing"
)

func TestLocateManaged(t *testing.T) {
	lines := []string{
		"# banner",
		"# TACK: backup",
		"0 2 * * * /usr/local/bin/backup.sh",
		"",
		"# TACK: report",
		"@daily /usr/bin/report",
	}
	m, ok := locateManaged(lines, "backup")
	if !ok || m.markerIdx != 1 || m.entryIdx != 2 {
		t.Fatalf("backup: got %+v ok=%v", m, ok)
	}
	m, ok = locateManaged(lines, "report")
	if !ok || m.markerIdx != 4 || m.entryIdx != 5 {
		t.Fatalf("report: got %+v ok=%v", m, ok)
	}
	if _, ok := locateManaged(lines, "missing"); ok {
		t.Fatalf("missing should not be found")
	}
}

func TestLocateManaged_MarkerAtEOF(t *testing.T) {
	lines := []string{"# TACK: dangling"}
	m, ok := locateManaged(lines, "dangling")
	if !ok || m.markerIdx != 0 || m.entryIdx != -1 {
		t.Fatalf("dangling: got %+v ok=%v", m, ok)
	}
}

func TestRenderSchedulePrefix(t *testing.T) {
	cases := []struct {
		name, special, min, h, d, mon, w string
		want                             string
	}{
		{"special", "daily", "*", "*", "*", "*", "*", "@daily "},
		{"fields", "", "0", "2", "*", "*", "*", "0 2 * * * "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderSchedulePrefix(tc.special, tc.min, tc.h, tc.d, tc.mon, tc.w)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestRenderEntryLine(t *testing.T) {
	// env mode
	if got := renderEntryLine(true, "", "", "PATH=/usr/bin", false); got != "PATH=/usr/bin" {
		t.Errorf("env: got %q", got)
	}
	// user crontab
	if got := renderEntryLine(false, "0 2 * * * ", "", "/bin/backup", false); got != "0 2 * * * /bin/backup" {
		t.Errorf("user: got %q", got)
	}
	// drop-in
	if got := renderEntryLine(false, "0 2 * * * ", "alice", "/bin/backup", true); got != "0 2 * * * alice /bin/backup" {
		t.Errorf("dropin: got %q", got)
	}
}

func TestRenderManagedBlock(t *testing.T) {
	marker, line := renderManagedBlock("backup", "0 2 * * * /bin/b", false)
	if marker != "# TACK: backup" || line != "0 2 * * * /bin/b" {
		t.Errorf("enabled: %q / %q", marker, line)
	}
	marker, line = renderManagedBlock("backup", "0 2 * * * /bin/b", true)
	if marker != "# TACK: backup" || line != "# 0 2 * * * /bin/b" {
		t.Errorf("disabled: %q / %q", marker, line)
	}
}

func TestApplyPresent_EmptyFile(t *testing.T) {
	out := applyPresent(nil, "backup", "# TACK: backup", "0 2 * * * /bin/b")
	want := []string{"# TACK: backup", "0 2 * * * /bin/b"}
	if !slicesEqual(out, want) {
		t.Errorf("got %#v want %#v", out, want)
	}
}

func TestApplyPresent_AppendSeparatesFromExistingContent(t *testing.T) {
	in := []string{"# existing", "0 1 * * * /other"}
	out := applyPresent(in, "backup", "# TACK: backup", "0 2 * * * /bin/b")
	want := []string{"# existing", "0 1 * * * /other", "", "# TACK: backup", "0 2 * * * /bin/b"}
	if !slicesEqual(out, want) {
		t.Errorf("got %#v\nwant %#v", out, want)
	}
}

func TestApplyPresent_ReplaceInPlace(t *testing.T) {
	in := []string{"# header", "# TACK: backup", "0 1 * * * /old", "# trailing"}
	out := applyPresent(in, "backup", "# TACK: backup", "0 2 * * * /new")
	want := []string{"# header", "# TACK: backup", "0 2 * * * /new", "# trailing"}
	if !slicesEqual(out, want) {
		t.Errorf("got %#v\nwant %#v", out, want)
	}
}

func TestApplyPresent_MarkerWithoutFollowingLine(t *testing.T) {
	in := []string{"# TACK: backup"}
	out := applyPresent(in, "backup", "# TACK: backup", "0 2 * * * /x")
	want := []string{"# TACK: backup", "0 2 * * * /x"}
	if !slicesEqual(out, want) {
		t.Errorf("got %#v want %#v", out, want)
	}
}

func TestApplyAbsent_Removes(t *testing.T) {
	in := []string{"# keep", "# TACK: backup", "0 2 * * * /bin/b", "# tail"}
	out, removed := applyAbsent(in, "backup")
	if !removed {
		t.Fatal("expected removed=true")
	}
	want := []string{"# keep", "# tail"}
	if !slicesEqual(out, want) {
		t.Errorf("got %#v want %#v", out, want)
	}
}

func TestApplyAbsent_NotFound(t *testing.T) {
	in := []string{"# keep"}
	out, removed := applyAbsent(in, "nope")
	if removed || !slicesEqual(out, in) {
		t.Errorf("unexpected: removed=%v out=%#v", removed, out)
	}
}

func TestApplyAbsent_LastEntryDeletesTrailingBlanks(t *testing.T) {
	in := []string{"# TACK: backup", "0 2 * * * /bin/b"}
	out, removed := applyAbsent(in, "backup")
	if !removed {
		t.Fatal("expected removed=true")
	}
	if len(out) != 0 {
		t.Errorf("expected empty, got %#v", out)
	}
}

func TestJoinSplitRoundtrip(t *testing.T) {
	cases := []string{
		"",
		"one\n",
		"one\ntwo\n",
		"one\ntwo\nthree\n",
	}
	for _, s := range cases {
		got := joinLines(splitLines(s))
		if got != s {
			t.Errorf("roundtrip %q -> %q", s, got)
		}
	}
}

func TestIsValidName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"backup", false},
		{"backup-job", false},
		{"a b c", false},
		{"", true},
		{"with\nnewline", true},
		{"with#hash", true},
		{"with\ttab", false}, // tab is printable-ish; but 0x09 < 0x20 so rejected
		{strings.Repeat("x", 200), false},
		{strings.Repeat("x", 201), true},
	}
	// tab (0x09) is < 0x20, so the test above expects it to FAIL — fix expectations:
	cases[6].wantErr = true
	for _, tc := range cases {
		err := isValidName(tc.name)
		if (err != nil) != tc.wantErr {
			t.Errorf("isValidName(%q) err=%v wantErr=%v", tc.name, err, tc.wantErr)
		}
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

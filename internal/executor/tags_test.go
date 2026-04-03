package executor

import (
	"testing"

	"github.com/eugenetaranov/bolt/internal/playbook"
)

func TestShouldRunTask(t *testing.T) {
	tests := []struct {
		name          string
		effectiveTags []string
		tags          []string
		skipTags      []string
		want          bool
	}{
		// No filters
		{"no filters, no tags", nil, nil, nil, true},
		{"no filters, has tags", []string{"deploy"}, nil, nil, true},
		{"no filters, never tag", []string{"never", "debug"}, nil, nil, false},

		// --tags filter (basic matching, OR logic)
		{"tags filter match", []string{"deploy"}, []string{"deploy"}, nil, true},
		{"tags filter no match", []string{"config"}, []string{"deploy"}, nil, false},
		{"tags filter OR match first", []string{"deploy"}, []string{"deploy", "config"}, nil, true},
		{"tags filter OR match second", []string{"config"}, []string{"deploy", "config"}, nil, true},
		{"tags filter no tags on task", nil, []string{"deploy"}, nil, false},
		{"tags filter multiple effective", []string{"deploy", "web"}, []string{"web"}, nil, true},

		// --skip-tags
		{"skip-tags match", []string{"debug"}, nil, []string{"debug"}, false},
		{"skip-tags no match", []string{"deploy"}, nil, []string{"debug"}, true},
		{"skip-tags one matches", []string{"deploy", "debug"}, nil, []string{"debug"}, false},

		// Combined --tags and --skip-tags
		{"tags+skip-tags: match both", []string{"deploy", "slow"}, []string{"deploy"}, []string{"slow"}, false},
		{"tags+skip-tags: match tag not skip", []string{"deploy"}, []string{"deploy"}, []string{"slow"}, true},

		// always tag
		{"always runs with no filter", []string{"always", "setup"}, nil, nil, true},
		{"always runs despite tags filter", []string{"always", "setup"}, []string{"deploy"}, nil, true},
		{"always skipped by skip-tags", []string{"always", "setup"}, []string{"deploy"}, []string{"always"}, false},

		// never tag
		{"never skipped by default", []string{"never", "debug"}, nil, nil, false},
		{"never runs when explicitly tagged", []string{"never", "debug"}, []string{"debug"}, nil, true},
		{"never skipped when only never in filter", []string{"never", "debug"}, []string{"never"}, nil, true},
		{"never with skip-tags", []string{"never", "debug"}, []string{"debug"}, []string{"debug"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRunTask(tt.effectiveTags, tt.tags, tt.skipTags)
			if got != tt.want {
				t.Errorf("shouldRunTask(%v, %v, %v) = %v, want %v",
					tt.effectiveTags, tt.tags, tt.skipTags, got, tt.want)
			}
		})
	}
}

func TestEffectiveTags(t *testing.T) {
	tests := []struct {
		name      string
		task      *playbook.Task
		playTags  []string
		blockTags []string
		want      []string
	}{
		{
			name:     "no tags",
			task:     &playbook.Task{},
			want:     nil,
		},
		{
			name:     "task tags only",
			task:     &playbook.Task{Tags: []string{"deploy"}},
			want:     []string{"deploy"},
		},
		{
			name:     "play + task tags",
			task:     &playbook.Task{Tags: []string{"nginx"}},
			playTags: []string{"infra"},
			want:     []string{"infra", "nginx"},
		},
		{
			name:      "play + block + task tags",
			task:      &playbook.Task{Tags: []string{"nginx"}},
			playTags:  []string{"infra"},
			blockTags: []string{"deploy"},
			want:      []string{"infra", "deploy", "nginx"},
		},
		{
			name:      "deduplication",
			task:      &playbook.Task{Tags: []string{"deploy", "infra"}},
			playTags:  []string{"infra"},
			blockTags: []string{"deploy"},
			want:      []string{"infra", "deploy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveTags(tt.task, tt.playTags, tt.blockTags)
			if len(got) != len(tt.want) {
				t.Fatalf("effectiveTags() = %v, want %v", got, tt.want)
			}
			for i, tag := range got {
				if tag != tt.want[i] {
					t.Errorf("effectiveTags()[%d] = %q, want %q", i, tag, tt.want[i])
				}
			}
		})
	}
}

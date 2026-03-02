package executor

import (
	"testing"
)

func TestParseConnectionURI(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ConnOverrides
		wantErr string
	}{
		// Plain strings — no parsing
		{name: "plain ssh", input: "ssh", want: nil},
		{name: "plain local", input: "local", want: nil},
		{name: "plain docker", input: "docker", want: nil},

		// SSH URIs
		{
			name:  "ssh host only",
			input: "ssh://myhost",
			want:  &ConnOverrides{Connection: "ssh", Hosts: []string{"myhost"}},
		},
		{
			name:  "ssh user@host",
			input: "ssh://deploy@myhost",
			want:  &ConnOverrides{Connection: "ssh", Hosts: []string{"myhost"}, SSHUser: "deploy"},
		},
		{
			name:  "ssh user@host:port",
			input: "ssh://deploy@myhost:2222",
			want:  &ConnOverrides{Connection: "ssh", Hosts: []string{"myhost:2222"}, SSHUser: "deploy", SSHPort: 2222},
		},
		{
			name:  "ssh user:pass@host:port",
			input: "ssh://deploy:s3cret@myhost:2222",
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"myhost:2222"},
				SSHUser: "deploy", SSHPass: "s3cret", HasSSHPass: true, SSHPort: 2222,
			},
		},
		{
			name:  "ssh password with @ sign",
			input: "ssh://deploy:p@ss@myhost:22",
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"myhost:22"},
				SSHUser: "deploy", SSHPass: "p@ss", HasSSHPass: true, SSHPort: 22,
			},
		},
		{
			name:  "ssh host:port no user",
			input: "ssh://myhost:8022",
			want:  &ConnOverrides{Connection: "ssh", Hosts: []string{"myhost:8022"}, SSHPort: 8022},
		},
		{
			name:  "ssh IPv6 host with port",
			input: "ssh://user@[::1]:2222",
			want:  &ConnOverrides{Connection: "ssh", Hosts: []string{"[::1]:2222"}, SSHUser: "user", SSHPort: 2222},
		},
		{
			name:  "ssh IPv6 host without port",
			input: "ssh://user@[::1]",
			want:  &ConnOverrides{Connection: "ssh", Hosts: []string{"::1"}, SSHUser: "user"},
		},

		// Docker URIs
		{
			name:  "docker container",
			input: "docker://my-container",
			want:  &ConnOverrides{Connection: "docker", Hosts: []string{"my-container"}},
		},

		// Local URIs
		{
			name:  "local explicit",
			input: "local://",
			want:  &ConnOverrides{Connection: "local"},
		},

		// SSM URIs — SSM targets come from --ssm-instances/--ssm-tags, not the URI path
		{name: "plain ssm", input: "ssm", want: nil},
		{
			name:  "ssm URI bare",
			input: "ssm://",
			want:  &ConnOverrides{Connection: "ssm"},
		},
		{
			name:  "ssm URI with path ignored",
			input: "ssm://i-0abc123",
			want:  &ConnOverrides{Connection: "ssm"},
		},

		// Error cases
		{name: "unsupported scheme", input: "ftp://host", wantErr: "unsupported connection scheme"},
		{name: "ssh missing host", input: "ssh://", wantErr: "requires a host"},
		{name: "ssh invalid port", input: "ssh://host:abc", wantErr: "invalid port"},
		{name: "ssh port out of range", input: "ssh://host:99999", wantErr: "port out of range"},
		{name: "docker missing container", input: "docker://", wantErr: "requires a container name"},
		{name: "local with host", input: "local://something", wantErr: "must not contain a host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConnectionURI(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.want)
			}

			assertOverridesEqual(t, tt.want, got)
		})
	}
}

func TestMergeConnectionURIs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    *ConnOverrides
		wantErr string
	}{
		{
			name:  "empty",
			input: []string{},
			want:  &ConnOverrides{},
		},
		{
			name:  "single plain string",
			input: []string{"ssh"},
			want:  &ConnOverrides{Connection: "ssh"},
		},
		{
			name:  "single URI",
			input: []string{"ssh://deploy@web1:2222"},
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"web1:2222"},
				SSHUser: "deploy", SSHPort: 2222,
			},
		},
		{
			name:  "multiple URIs accumulate hosts",
			input: []string{"ssh://deploy@web1:2222", "ssh://deploy@web2:2222"},
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"web1:2222", "web2:2222"},
				SSHUser: "deploy", SSHPort: 2222,
			},
		},
		{
			name:  "last non-empty user wins",
			input: []string{"ssh://alice@web1", "ssh://bob@web2"},
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"web1", "web2"},
				SSHUser: "bob",
			},
		},
		{
			name:  "last non-empty port wins",
			input: []string{"ssh://web1:2222", "ssh://web2:3333"},
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"web1:2222", "web2:3333"},
				SSHPort: 3333,
			},
		},
		{
			name:  "different ports per host are preserved",
			input: []string{"ssh://user@127.0.0.1:2201", "ssh://user@127.0.0.1:2202", "ssh://user@127.0.0.1:2203"},
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"127.0.0.1:2201", "127.0.0.1:2202", "127.0.0.1:2203"},
				SSHUser: "user", SSHPort: 2203,
			},
		},
		{
			name:  "plain ssh plus URI adds host",
			input: []string{"ssh", "ssh://web1"},
			want: &ConnOverrides{
				Connection: "ssh", Hosts: []string{"web1"},
			},
		},
		{
			name:  "plain ssm",
			input: []string{"ssm"},
			want:  &ConnOverrides{Connection: "ssm"},
		},
		{
			name:    "mixed ssh and ssm error",
			input:   []string{"ssh://web1", "ssm://"},
			wantErr: "mixed connection schemes",
		},
		{
			name:    "mixed schemes error",
			input:   []string{"ssh://web1", "docker://container"},
			wantErr: "mixed connection schemes",
		},
		{
			name:    "mixed plain schemes error",
			input:   []string{"ssh", "docker"},
			wantErr: "mixed connection schemes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeConnectionURIs(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertOverridesEqual(t, tt.want, got)
		})
	}
}

func assertOverridesEqual(t *testing.T, want, got *ConnOverrides) {
	t.Helper()

	if got.Connection != want.Connection {
		t.Errorf("Connection: got %q, want %q", got.Connection, want.Connection)
	}
	if len(got.Hosts) != len(want.Hosts) {
		t.Errorf("Hosts: got %v, want %v", got.Hosts, want.Hosts)
	} else {
		for i := range want.Hosts {
			if got.Hosts[i] != want.Hosts[i] {
				t.Errorf("Hosts[%d]: got %q, want %q", i, got.Hosts[i], want.Hosts[i])
			}
		}
	}
	if got.SSHUser != want.SSHUser {
		t.Errorf("SSHUser: got %q, want %q", got.SSHUser, want.SSHUser)
	}
	if got.SSHPort != want.SSHPort {
		t.Errorf("SSHPort: got %d, want %d", got.SSHPort, want.SSHPort)
	}
	if got.SSHPass != want.SSHPass {
		t.Errorf("SSHPass: got %q, want %q", got.SSHPass, want.SSHPass)
	}
	if got.HasSSHPass != want.HasSSHPass {
		t.Errorf("HasSSHPass: got %v, want %v", got.HasSSHPass, want.HasSSHPass)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

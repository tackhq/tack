package executor

import "testing"

func TestFormatApprovalTarget(t *testing.T) {
	tests := []struct {
		name       string
		hosts      []string
		connection string
		want       string
	}{
		{
			name:       "single host with ssh connection",
			hosts:      []string{"web1.prod"},
			connection: "ssh",
			want:       "web1.prod (ssh)",
		},
		{
			name:       "single SSM instance ID",
			hosts:      []string{"i-0a1b2c3d4e5f"},
			connection: "ssm",
			want:       "i-0a1b2c3d4e5f (ssm)",
		},
		{
			name:       "single host with empty connection defaults to local",
			hosts:      []string{"localhost"},
			connection: "",
			want:       "localhost (local)",
		},
		{
			name:       "two hosts inline",
			hosts:      []string{"web1", "web2"},
			connection: "ssh",
			want:       "2 hosts (web1, web2)",
		},
		{
			name:       "exactly five hosts inline",
			hosts:      []string{"web1", "web2", "web3", "web4", "web5"},
			connection: "ssh",
			want:       "5 hosts (web1, web2, web3, web4, web5)",
		},
		{
			name:       "six hosts triggers truncation",
			hosts:      []string{"web1", "web2", "web3", "web4", "web5", "web6"},
			connection: "ssh",
			want:       "6 hosts (web1, web2, web3, web4, web5, ...)",
		},
		{
			name:       "twelve hosts truncated to first five",
			hosts:      []string{"web1", "web2", "web3", "web4", "web5", "web6", "web7", "web8", "web9", "web10", "web11", "web12"},
			connection: "ssh",
			want:       "12 hosts (web1, web2, web3, web4, web5, ...)",
		},
		{
			name:       "empty hosts (defensive)",
			hosts:      nil,
			connection: "local",
			want:       "(no hosts) (local)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatApprovalTarget(tt.hosts, tt.connection)
			if got != tt.want {
				t.Errorf("formatApprovalTarget(%v, %q) = %q; want %q", tt.hosts, tt.connection, got, tt.want)
			}
		})
	}
}

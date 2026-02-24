package executor

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// validSchemes lists the connection URI schemes we support.
var validSchemes = map[string]bool{
	"ssh":    true,
	"docker": true,
	"local":  true,
}

// ParseConnectionURI parses a single connection URI string.
// If the string contains no "://", it returns nil, nil (plain string like "ssh" or "local").
// Otherwise it validates the scheme and extracts host, user, password, and port.
func ParseConnectionURI(s string) (*ConnOverrides, error) {
	idx := strings.Index(s, "://")
	if idx < 0 {
		return nil, nil // plain string, caller handles
	}

	scheme := s[:idx]
	rest := s[idx+3:]

	if !validSchemes[scheme] {
		return nil, fmt.Errorf("unsupported connection scheme: %q", scheme)
	}

	o := &ConnOverrides{Connection: scheme}

	switch scheme {
	case "local":
		if rest != "" {
			return nil, fmt.Errorf("local:// URI must not contain a host component")
		}
		return o, nil

	case "docker":
		if rest == "" {
			return nil, fmt.Errorf("docker:// URI requires a container name")
		}
		o.Hosts = []string{rest}
		return o, nil

	case "ssh":
		return parseSSHURI(o, rest)
	}

	return o, nil
}

// parseSSHURI parses the host portion of an ssh:// URI into the ConnOverrides.
func parseSSHURI(o *ConnOverrides, rest string) (*ConnOverrides, error) {
	if rest == "" {
		return nil, fmt.Errorf("ssh:// URI requires a host")
	}

	var userinfo, hostport string

	// Split userinfo from host — use last "@" so passwords can contain "@"
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		userinfo = rest[:at]
		hostport = rest[at+1:]
	} else {
		hostport = rest
	}

	// Parse userinfo (user or user:pass)
	if userinfo != "" {
		if ci := strings.Index(userinfo, ":"); ci >= 0 {
			o.SSHUser = userinfo[:ci]
			o.SSHPass = userinfo[ci+1:]
			o.HasSSHPass = true
		} else {
			o.SSHUser = userinfo
		}
	}

	// Parse host:port
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}

	if host == "" {
		return nil, fmt.Errorf("ssh:// URI requires a host")
	}

	if port != 0 {
		o.Hosts = []string{net.JoinHostPort(host, strconv.Itoa(port))}
		o.SSHPort = port
	} else {
		o.Hosts = []string{host}
	}

	return o, nil
}

// splitHostPort splits a host:port string, handling IPv6 bracket notation.
// Returns the host, port (0 if not specified), and any error.
func splitHostPort(s string) (string, int, error) {
	// IPv6: [::1]:port or [::1]
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", 0, fmt.Errorf("invalid IPv6 address: missing closing bracket")
		}
		host := s[1:end]
		remaining := s[end+1:]
		if remaining == "" {
			return host, 0, nil
		}
		if !strings.HasPrefix(remaining, ":") {
			return "", 0, fmt.Errorf("invalid host:port format after IPv6 address")
		}
		port, err := parsePort(remaining[1:])
		if err != nil {
			return "", 0, err
		}
		return host, port, nil
	}

	// Regular host:port — last colon separates host from port
	if ci := strings.LastIndex(s, ":"); ci >= 0 {
		portStr := s[ci+1:]
		if portStr != "" {
			port, err := parsePort(portStr)
			if err != nil {
				return "", 0, err
			}
			return s[:ci], port, nil
		}
	}

	return s, 0, nil
}

// parsePort parses and validates a port string.
func parsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %q", s)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return port, nil
}

// MergeConnectionURIs parses multiple -c values and merges them.
// All URIs must share the same scheme (or all be plain strings).
// Hosts accumulate across URIs; for SSH fields, the last non-empty value wins.
func MergeConnectionURIs(uris []string) (*ConnOverrides, error) {
	if len(uris) == 0 {
		return &ConnOverrides{}, nil
	}

	merged := &ConnOverrides{}
	var scheme string // track scheme for consistency check

	for _, raw := range uris {
		parsed, err := ParseConnectionURI(raw)
		if err != nil {
			return nil, err
		}

		if parsed == nil {
			// Plain string like "ssh", "local", "docker"
			thisScheme := raw
			if scheme == "" {
				scheme = thisScheme
			} else if scheme != thisScheme {
				return nil, fmt.Errorf("mixed connection schemes: %q and %q", scheme, thisScheme)
			}
			merged.Connection = thisScheme
			continue
		}

		// URI with scheme
		thisScheme := parsed.Connection
		if scheme == "" {
			scheme = thisScheme
		} else if scheme != thisScheme {
			return nil, fmt.Errorf("mixed connection schemes: %q and %q", scheme, thisScheme)
		}

		merged.Connection = thisScheme
		merged.Hosts = append(merged.Hosts, parsed.Hosts...)

		if parsed.SSHUser != "" {
			merged.SSHUser = parsed.SSHUser
		}
		if parsed.SSHPort != 0 {
			merged.SSHPort = parsed.SSHPort
		}
		if parsed.HasSSHPass {
			merged.HasSSHPass = true
			merged.SSHPass = parsed.SSHPass
		}
	}

	return merged, nil
}

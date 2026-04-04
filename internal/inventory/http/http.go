// Package http implements an inventory plugin that fetches inventory data
// from an HTTP endpoint.
package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/eugenetaranov/bolt/internal/inventory"
)

func init() {
	inventory.RegisterPlugin(&Plugin{})
}

// Plugin implements the inventory.Plugin interface for HTTP-based inventory.
type Plugin struct{}

func (p *Plugin) Name() string { return "http" }

func (p *Plugin) Load(ctx context.Context, config map[string]any) (*inventory.Inventory, error) {
	url, _ := config["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("http plugin: missing required 'url' config")
	}

	// Interpolate env vars in all string config values
	url = interpolateEnv(url)

	// Build request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http plugin: invalid URL: %w", err)
	}

	// Add query params
	if params, ok := config["params"].(map[string]any); ok {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		req.URL.RawQuery = q.Encode()
	}

	// Add headers
	if headers, ok := config["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, interpolateEnv(fmt.Sprintf("%v", v)))
		}
	}

	// Add basic auth
	if auth, ok := config["auth"].(map[string]any); ok {
		if basic, ok := auth["basic"].(map[string]any); ok {
			user := interpolateEnv(fmt.Sprintf("%v", basic["username"]))
			pass := interpolateEnv(fmt.Sprintf("%v", basic["password"]))
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
		}
	}

	// Build HTTP client with TLS config
	transport := &http.Transport{}
	if tlsCfg, ok := config["tls"].(map[string]any); ok {
		tc, err := buildTLSConfig(tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("http plugin: %w", err)
		}
		transport.TLSClientConfig = tc
	}

	// Timeout
	timeout := 30 * time.Second
	if t, ok := config["timeout"]; ok {
		switch v := t.(type) {
		case int:
			timeout = time.Duration(v) * time.Second
		case float64:
			timeout = time.Duration(v) * time.Second
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http plugin: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("http plugin: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(body)
		if len(preview) > 1024 {
			preview = preview[:1024] + "..."
		}
		return nil, fmt.Errorf("http plugin: server returned %d: %s", resp.StatusCode, preview)
	}

	inv, err := inventory.ParseInventoryData(body)
	if err != nil {
		return nil, fmt.Errorf("http plugin: %w", err)
	}

	return inv, nil
}

func buildTLSConfig(cfg map[string]any) (*tls.Config, error) {
	tc := &tls.Config{}

	if insecure, ok := cfg["insecure_skip_verify"].(bool); ok && insecure {
		tc.InsecureSkipVerify = true
	}

	if caPath, ok := cfg["ca_cert"].(string); ok && caPath != "" {
		caCert, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca_cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse ca_cert")
		}
		tc.RootCAs = pool
	}

	clientCert, hasCert := cfg["client_cert"].(string)
	clientKey, hasKey := cfg["client_key"].(string)
	if hasCert && hasKey && clientCert != "" && clientKey != "" {
		cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %w", err)
		}
		tc.Certificates = []tls.Certificate{cert}
	}

	return tc, nil
}

var envVarRe = regexp.MustCompile(`\{\{\s*env\.(\w+)\s*\}\}`)

func interpolateEnv(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := envVarRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		val := os.Getenv(parts[1])
		if val == "" {
			return match
		}
		return strings.Replace(match, match, val, 1)
	})
}

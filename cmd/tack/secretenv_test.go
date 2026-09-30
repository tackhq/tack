package main

import (
	"os"
	"testing"
)

func TestCaptureSecretEnv_RemovesFromEnvironment(t *testing.T) {
	t.Setenv("TACK_SUDO_PASSWORD", "sudo-secret")
	t.Setenv("TACK_SSH_PASSWORD", "ssh-secret")
	t.Setenv("TACK_VAULT_PASSWORD", "vault-secret")
	t.Cleanup(func() { capturedSecretEnv = nil })

	captureSecretEnv()

	for name, want := range map[string]string{
		"TACK_SUDO_PASSWORD":  "sudo-secret",
		"TACK_SSH_PASSWORD":   "ssh-secret",
		"TACK_VAULT_PASSWORD": "vault-secret",
	} {
		if _, ok := os.LookupEnv(name); ok {
			t.Errorf("%s still in the process environment; children would inherit it", name)
		}
		if got := secretEnv(name); got != want {
			t.Errorf("secretEnv(%s) = %q, want %q", name, got, want)
		}
	}
}

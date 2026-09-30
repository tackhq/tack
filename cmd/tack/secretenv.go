package main

import "os"

// secretEnvVars are environment variables that carry credentials. They are
// captured once at startup and removed from the process environment so they
// are not inherited by child processes (local `command:` tasks, installers,
// brew post-install scripts, ...).
var secretEnvVars = []string{
	"TACK_SUDO_PASSWORD",
	"TACK_SSH_PASSWORD",
	"TACK_VAULT_PASSWORD",
}

// capturedSecretEnv holds the values captured by captureSecretEnv. A nil map
// means capture has not run (e.g. in tests), and secretEnv falls back to
// os.Getenv.
var capturedSecretEnv map[string]string

// captureSecretEnv moves credential env vars from the process environment
// into memory.
func captureSecretEnv() {
	capturedSecretEnv = make(map[string]string, len(secretEnvVars))
	for _, name := range secretEnvVars {
		if v, ok := os.LookupEnv(name); ok {
			capturedSecretEnv[name] = v
			_ = os.Unsetenv(name)
		}
	}
}

// secretEnv returns the value of a credential env var, reading the captured
// copy when captureSecretEnv has run.
func secretEnv(name string) string {
	if capturedSecretEnv != nil {
		return capturedSecretEnv[name]
	}
	return os.Getenv(name)
}

package docker

import (
	"testing"
)

func TestSetSudoEnablesRoot(t *testing.T) {
	c := New("test-container")
	c.SetSudo(true, "")

	args := c.buildExecArgs("whoami")
	found := false
	for i, arg := range args {
		if arg == "-u" && i+1 < len(args) && args[i+1] == "root" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -u root in args, got %v", args)
	}
}

func TestSetSudoDisableRevertsUser(t *testing.T) {
	c := New("test-container", WithUser("appuser"))
	c.SetSudo(true, "")
	c.SetSudo(false, "")

	args := c.buildExecArgs("whoami")
	found := false
	for i, arg := range args {
		if arg == "-u" && i+1 < len(args) && args[i+1] == "appuser" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -u appuser after sudo disable, got %v", args)
	}
}

func TestSetSudoTogglePreservesCustomUser(t *testing.T) {
	c := New("test-container", WithUser("deploy"))

	// Enable sudo → root
	c.SetSudo(true, "")
	args := c.buildExecArgs("id")
	assertUser(t, args, "root")

	// Disable sudo → back to deploy
	c.SetSudo(false, "")
	args = c.buildExecArgs("id")
	assertUser(t, args, "deploy")

	// Enable again → root
	c.SetSudo(true, "")
	args = c.buildExecArgs("id")
	assertUser(t, args, "root")
}

func TestSetSudoNoUserDefault(t *testing.T) {
	c := New("test-container") // no WithUser

	c.SetSudo(true, "")
	args := c.buildExecArgs("id")
	assertUser(t, args, "root")

	c.SetSudo(false, "")
	args = c.buildExecArgs("id")
	// No -u flag should be present (empty original user)
	for i, arg := range args {
		if arg == "-u" && i+1 < len(args) {
			t.Errorf("expected no -u flag after sudo disable with no original user, got -u %s", args[i+1])
		}
	}
}

func TestSetSudoPasswordAccepted(t *testing.T) {
	c := New("test-container")
	// Should not panic or error
	c.SetSudo(true, "somepassword")
	if !c.sudoEnabled {
		t.Error("expected sudoEnabled=true")
	}
	if c.user != "root" {
		t.Errorf("expected user=root, got %s", c.user)
	}
}

func assertUser(t *testing.T, args []string, expected string) {
	t.Helper()
	for i, arg := range args {
		if arg == "-u" && i+1 < len(args) {
			if args[i+1] != expected {
				t.Errorf("expected -u %s, got -u %s", expected, args[i+1])
			}
			return
		}
	}
	t.Errorf("expected -u %s in args, not found: %v", expected, args)
}

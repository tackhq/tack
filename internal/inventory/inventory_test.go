package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

const sshInventory = `
hosts:
  web1:
    ssh:
      user: deploy
      port: 2222
      key: ~/.ssh/id_deploy
    vars:
      region: us-east-1
      role: web

  web2:
    vars:
      region: us-west-2
      role: web

  db1:
    ssh:
      user: postgres
      host_key_checking: false
    vars:
      role: db

groups:
  webservers:
    hosts: [web1, web2]
    ssh:
      user: deploy
    vars:
      app_port: 8080
`

const ssmInventory = `
hosts:
  i-0abc1234:
    vars:
      env: production

groups:
  prod-app:
    connection: ssm
    ssm:
      region: us-east-1
      bucket: my-s3-bucket
    hosts: [i-0abc1234, i-0def5678]
    vars:
      env: production
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "inventory-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_SSH(t *testing.T) {
	path := writeTemp(t, sshInventory)
	inv, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Hosts
	if len(inv.Hosts) != 3 {
		t.Errorf("Hosts len = %d, want 3", len(inv.Hosts))
	}

	web1 := inv.Hosts["web1"]
	if web1 == nil {
		t.Fatal("web1 not found")
	}
	if web1.SSH == nil {
		t.Fatal("web1.SSH is nil")
	}
	if web1.SSH.User != "deploy" {
		t.Errorf("web1.SSH.User = %q, want deploy", web1.SSH.User)
	}
	if web1.SSH.Port != 2222 {
		t.Errorf("web1.SSH.Port = %d, want 2222", web1.SSH.Port)
	}
	if web1.SSH.Key != "~/.ssh/id_deploy" {
		t.Errorf("web1.SSH.Key = %q, want ~/.ssh/id_deploy", web1.SSH.Key)
	}
	if web1.Vars["region"] != "us-east-1" {
		t.Errorf("web1.Vars[region] = %v, want us-east-1", web1.Vars["region"])
	}

	web2 := inv.Hosts["web2"]
	if web2 == nil {
		t.Fatal("web2 not found")
	}
	if web2.SSH != nil {
		t.Error("web2.SSH should be nil (no ssh block)")
	}

	db1 := inv.Hosts["db1"]
	if db1 == nil {
		t.Fatal("db1 not found")
	}
	if db1.SSH == nil || db1.SSH.HostKeyChecking == nil || *db1.SSH.HostKeyChecking != false {
		t.Error("db1.SSH.HostKeyChecking should be false")
	}

	// Groups
	if len(inv.Groups) != 1 {
		t.Errorf("Groups len = %d, want 1", len(inv.Groups))
	}
	wg := inv.Groups["webservers"]
	if wg == nil {
		t.Fatal("webservers group not found")
	}
	if len(wg.Hosts) != 2 {
		t.Errorf("webservers.Hosts len = %d, want 2", len(wg.Hosts))
	}
	if wg.SSH == nil || wg.SSH.User != "deploy" {
		t.Errorf("webservers.SSH.User = %v, want deploy", wg.SSH)
	}
	if wg.Vars["app_port"] != 8080 {
		t.Errorf("webservers.Vars[app_port] = %v, want 8080", wg.Vars["app_port"])
	}
}

func TestLoad_SSM(t *testing.T) {
	path := writeTemp(t, ssmInventory)
	inv, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	g := inv.Groups["prod-app"]
	if g == nil {
		t.Fatal("prod-app group not found")
	}
	if g.Connection != "ssm" {
		t.Errorf("Connection = %q, want ssm", g.Connection)
	}
	if g.SSM == nil {
		t.Fatal("SSM config is nil")
	}
	if g.SSM.Region != "us-east-1" {
		t.Errorf("SSM.Region = %q, want us-east-1", g.SSM.Region)
	}
	if g.SSM.Bucket != "my-s3-bucket" {
		t.Errorf("SSM.Bucket = %q, want my-s3-bucket", g.SSM.Bucket)
	}
	if len(g.Hosts) != 2 {
		t.Errorf("prod-app.Hosts len = %d, want 2", len(g.Hosts))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestExpandGroup(t *testing.T) {
	path := writeTemp(t, sshInventory)
	inv, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Known group
	hosts, group, ok := inv.ExpandGroup("webservers")
	if !ok {
		t.Error("webservers: expected ok=true")
	}
	if group == nil {
		t.Error("webservers: expected non-nil group")
	}
	if len(hosts) != 2 {
		t.Errorf("webservers hosts len = %d, want 2", len(hosts))
	}

	// Known host (not a group)
	hosts, group, ok = inv.ExpandGroup("web1")
	if !ok {
		t.Error("web1: expected ok=true")
	}
	if group != nil {
		t.Error("web1: expected nil group")
	}
	if len(hosts) != 1 || hosts[0] != "web1" {
		t.Errorf("web1 hosts = %v, want [web1]", hosts)
	}

	// Unknown
	hosts, group, ok = inv.ExpandGroup("unknown-host")
	if ok {
		t.Error("unknown-host: expected ok=false")
	}
	if hosts != nil || group != nil {
		t.Error("unknown-host: expected nil results")
	}
}

func TestGetHostGroups(t *testing.T) {
	path := writeTemp(t, sshInventory)
	inv, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	groups := inv.GetHostGroups("web1")
	if len(groups) != 1 {
		t.Errorf("web1 groups len = %d, want 1", len(groups))
	}

	groups = inv.GetHostGroups("db1")
	if len(groups) != 0 {
		t.Errorf("db1 groups len = %d, want 0", len(groups))
	}
}

func TestNilInventory(t *testing.T) {
	var inv *Inventory

	hosts, group, ok := inv.ExpandGroup("anything")
	if ok || hosts != nil || group != nil {
		t.Error("nil inventory ExpandGroup should return false/nil")
	}

	if inv.GetHost("anything") != nil {
		t.Error("nil inventory GetHost should return nil")
	}

	if groups := inv.GetHostGroups("anything"); groups != nil {
		t.Error("nil inventory GetHostGroups should return nil")
	}
}

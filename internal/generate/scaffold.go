package generate

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldRole creates a new role directory structure with sample files.
// It returns an error if the role directory already exists.
func ScaffoldRole(name, basePath string) error {
	roleDir := filepath.Join(basePath, name)

	if _, err := os.Stat(roleDir); err == nil {
		return fmt.Errorf("role directory already exists: %s", roleDir)
	}

	dirs := []string{"tasks", "handlers", "defaults", "vars", "files", "templates"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(roleDir, d), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	files := map[string]string{
		"tasks/main.yaml":       tasksContent(name),
		"handlers/main.yaml":    handlersContent,
		"defaults/main.yaml":    defaultsContent(name),
		"vars/main.yaml":        varsContent,
		"files/config.txt":      filesContent,
		"templates/app.conf.j2": templateContent,
	}

	for relPath, content := range files {
		fullPath := filepath.Join(roleDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", relPath, err)
		}
	}

	return nil
}

func tasksContent(name string) string {
	return `# Tasks for role: ` + name + `
#
# These sample tasks demonstrate the available modules.

- name: Install packages (apt)
  apt:
    name: "{{ app_package }}"
    state: present
  when: tack_os == "linux"

- name: Install packages (brew)
  brew:
    name: "{{ app_package }}"
    state: present
  when: tack_os == "darwin"

- name: Create config directory
  file:
    path: "{{ config_dir }}"
    state: directory
    mode: "0755"

- name: Copy static config file
  copy:
    src: config.txt
    dest: "{{ config_dir }}/config.txt"
    mode: "0644"

- name: Render application config from template
  template:
    src: app.conf.j2
    dest: "{{ config_dir }}/app.conf"
    mode: "0644"

- name: Enable and start service
  systemd:
    name: "{{ service_name }}"
    state: started
    enabled: true
  when: tack_os == "linux"
  notify: restart service

- name: Verify service is listening
  command:
    cmd: "echo '` + name + ` role applied successfully'"
`
}

const handlersContent = `# Handlers are triggered by 'notify' in tasks.

- name: restart service
  systemd:
    name: "{{ service_name }}"
    state: restarted
`

func defaultsContent(name string) string {
	return `# Default variables for role: ` + name + `
# These can be overridden in playbook vars or extra-vars.

app_package: ` + name + `
config_dir: /etc/` + name + `
service_name: ` + name + `
`
}

const varsContent = `# Role variables (higher precedence than defaults).
# Add role-specific variables here.
`

const filesContent = `# Sample static configuration file.
# This file is deployed by the copy module as-is.
`

const templateContent = `# Application config for {{ .app_package }}
# Rendered by the template module using Go text/template syntax.

[app]
name = {{ .app_package }}
config_dir = {{ .config_dir }}
`

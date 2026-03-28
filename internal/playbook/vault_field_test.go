package playbook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPlayVaultFileField(t *testing.T) {
	t.Run("parses vault_file into VaultFile field", func(t *testing.T) {
		input := `
- name: test play
  hosts:
    - localhost
  vault_file: secrets.vault
  tasks: []
`
		var plays []*Play
		require.NoError(t, yaml.Unmarshal([]byte(input), &plays))
		require.Len(t, plays, 1)
		assert.Equal(t, "secrets.vault", plays[0].VaultFile)
	})

	t.Run("VaultFile is empty when not set", func(t *testing.T) {
		input := `
- name: test play
  hosts:
    - localhost
  tasks: []
`
		var plays []*Play
		require.NoError(t, yaml.Unmarshal([]byte(input), &plays))
		require.Len(t, plays, 1)
		assert.Equal(t, "", plays[0].VaultFile)
	})
}

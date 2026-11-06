package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestNewConfigQuotas(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		env        map[string]string
		wantNodes  int
		wantAdmins int64
		wantError  string
	}{
		{name: "defaults", wantNodes: 300, wantAdmins: 1},
		{name: "configured", env: map[string]string{"PANDAWIKI_MAX_NODES": "301", "PANDAWIKI_MAX_ADMINS": "2"}, wantNodes: 301, wantAdmins: 2},
		{name: "yaml", yaml: "quota:\n  max_nodes: 500\n  max_admins: 10\n", wantNodes: 500, wantAdmins: 10},
		{name: "environment overrides yaml", yaml: "quota:\n  max_nodes: 500\n  max_admins: 10\n", env: map[string]string{"PANDAWIKI_MAX_NODES": "600"}, wantNodes: 600, wantAdmins: 10},
		{name: "zero nodes", env: map[string]string{"PANDAWIKI_MAX_NODES": "0"}, wantError: "PANDAWIKI_MAX_NODES"},
		{name: "negative nodes", env: map[string]string{"PANDAWIKI_MAX_NODES": "-1"}, wantError: "PANDAWIKI_MAX_NODES"},
		{name: "invalid nodes", env: map[string]string{"PANDAWIKI_MAX_NODES": "many"}, wantError: "PANDAWIKI_MAX_NODES"},
		{name: "fractional nodes", env: map[string]string{"PANDAWIKI_MAX_NODES": "2.5"}, wantError: "PANDAWIKI_MAX_NODES"},
		{name: "overflow nodes", env: map[string]string{"PANDAWIKI_MAX_NODES": "9223372036854775808"}, wantError: "PANDAWIKI_MAX_NODES"},
		{name: "empty nodes", env: map[string]string{"PANDAWIKI_MAX_NODES": ""}, wantError: "PANDAWIKI_MAX_NODES"},
		{name: "zero admins", env: map[string]string{"PANDAWIKI_MAX_ADMINS": "0"}, wantError: "PANDAWIKI_MAX_ADMINS"},
		{name: "negative admins", env: map[string]string{"PANDAWIKI_MAX_ADMINS": "-1"}, wantError: "PANDAWIKI_MAX_ADMINS"},
		{name: "invalid admins", env: map[string]string{"PANDAWIKI_MAX_ADMINS": "many"}, wantError: "PANDAWIKI_MAX_ADMINS"},
		{name: "overflow admins", env: map[string]string{"PANDAWIKI_MAX_ADMINS": "9223372036854775808"}, wantError: "PANDAWIKI_MAX_ADMINS"},
		{name: "empty admins", env: map[string]string{"PANDAWIKI_MAX_ADMINS": ""}, wantError: "PANDAWIKI_MAX_ADMINS"},
		{name: "invalid yaml quota", yaml: "quota:\n  max_nodes: 0\n", wantError: "max_nodes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			viper.Reset()
			t.Cleanup(viper.Reset)
			for _, key := range []string{"PANDAWIKI_MAX_NODES", "PANDAWIKI_MAX_ADMINS"} {
				t.Setenv(key, "")
				require.NoError(t, os.Unsetenv(key))
			}
			if test.yaml != "" {
				require.NoError(t, os.WriteFile("config.yml", []byte(test.yaml), 0o600))
			}
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			cfg, err := NewConfig()
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				require.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantNodes, cfg.Quota.MaxNodes)
			require.Equal(t, test.wantAdmins, cfg.Quota.MaxAdmins)
		})
	}
}

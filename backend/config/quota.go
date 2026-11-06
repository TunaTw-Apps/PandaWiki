package config

import (
	"fmt"
	"os"
	"strconv"
)

// QuotaConfig controls the open-source edition's document and backend-account quotas.
type QuotaConfig struct {
	MaxNodes  int   `mapstructure:"max_nodes"`
	MaxAdmins int64 `mapstructure:"max_admins"`
}

func overrideQuotaWithEnv(quota *QuotaConfig) error {
	if value, ok := os.LookupEnv("PANDAWIKI_MAX_NODES"); ok {
		maxNodes, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid PANDAWIKI_MAX_NODES: %w", err)
		}
		quota.MaxNodes = maxNodes
	}
	if value, ok := os.LookupEnv("PANDAWIKI_MAX_ADMINS"); ok {
		maxAdmins, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid PANDAWIKI_MAX_ADMINS: %w", err)
		}
		quota.MaxAdmins = maxAdmins
	}
	if quota.MaxNodes <= 0 {
		return fmt.Errorf("quota.max_nodes / PANDAWIKI_MAX_NODES must be greater than zero")
	}
	if quota.MaxAdmins <= 0 {
		return fmt.Errorf("quota.max_admins / PANDAWIKI_MAX_ADMINS must be greater than zero")
	}
	return nil
}

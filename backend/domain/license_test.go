package domain

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaseEditionQuotas(t *testing.T) {
	defaults := GetBaseEditionLimitation(context.Background())
	require.Equal(t, 300, defaults.MaxNode)
	require.Equal(t, int64(1), defaults.MaxAdmin)

	ctx := WithBaseEditionQuotas(context.Background(), 301, 2)
	expected := defaults
	expected.MaxNode = 301
	expected.MaxAdmin = 2
	require.Equal(t, expected, GetBaseEditionLimitation(ctx))
	require.Equal(t, defaults, GetBaseEditionLimitation(context.Background()))

	// Deployment quotas must not turn on unrelated edition features.
	require.Equal(t, 1, expected.MaxKb)
	require.False(t, expected.AllowAdminPerm)

	for _, invalidEdition := range []any{[]byte("invalid JSON"), "wrong type"} {
		invalidCtx := context.WithValue(ctx, ContextKeyEditionLimitation, invalidEdition)
		require.Equal(t, expected, GetBaseEditionLimitation(invalidCtx))
	}
}

func TestEditionLimitationOverridesDeploymentQuotas(t *testing.T) {
	edition := BaseEditionLimitation{
		MaxKb:          10,
		MaxNode:        10000,
		MaxAdmin:       20,
		AllowAdminPerm: true,
	}
	encoded, err := json.Marshal(edition)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), ContextKeyEditionLimitation, encoded)
	ctx = WithBaseEditionQuotas(ctx, 301, 2)
	require.Equal(t, edition, GetBaseEditionLimitation(ctx))
}

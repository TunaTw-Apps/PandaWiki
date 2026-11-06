package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
)

func TestBaseEditionQuotaMiddleware(t *testing.T) {
	e := echo.New()
	e.Use(baseEditionQuotas(config.QuotaConfig{MaxNodes: 301, MaxAdmins: 2}))
	e.GET("/quota", func(c echo.Context) error {
		return c.JSON(http.StatusOK, domain.GetBaseEditionLimitation(c.Request().Context()))
	})

	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/quota", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var limits domain.BaseEditionLimitation
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &limits))
	require.Equal(t, 301, limits.MaxNode)
	require.Equal(t, int64(2), limits.MaxAdmin)
	require.Equal(t, 1, limits.MaxKb)
	require.False(t, limits.AllowAdminPerm)

	// Existing edition metadata survives the middleware and keeps its priority.
	request := httptest.NewRequest(http.MethodGet, "/quota", nil)
	request = request.WithContext(context.WithValue(request.Context(), domain.ContextKeyEditionLimitation,
		[]byte(`{"max_node":10000,"max_admin":20,"allow_admin_perm":true}`)))
	recorder = httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &limits))
	require.Equal(t, 10000, limits.MaxNode)
	require.Equal(t, int64(20), limits.MaxAdmin)
	require.True(t, limits.AllowAdminPerm)
}

package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatusIncludesCustomCurrencyIcon(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previous := *settings
	t.Cleanup(func() { *settings = previous })
	settings.QuotaDisplayType = operation_setting.QuotaDisplayTypeCustom
	settings.CustomCurrencySymbol = "Credits"
	for _, icon := range []string{"", "/api/appearance/background/coin.png"} {
		settings.CustomCurrencyIcon = icon
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
		GetStatus(c)
		var payload struct {
			Success bool           `json:"success"`
			Data    map[string]any `json:"data"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
		require.True(t, payload.Success)
		assert.Equal(t, icon, payload.Data["custom_currency_icon"])
		assert.Equal(t, "Credits", payload.Data["custom_currency_symbol"])
	}
}

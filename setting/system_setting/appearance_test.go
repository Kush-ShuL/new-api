package system_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppearanceSettings(t *testing.T) {
	defaults, err := ParseAppearanceSettings("")
	require.NoError(t, err)
	assert.Equal(t, AppearanceSettings{ImageOpacity: 85, MaskOpacity: 35, SurfaceOpacity: 85, BlurStrength: 20}, defaults)
	value, err := ParseAppearanceSettings(`{"glassEnabled":true,"background":"data:image/png;base64,image","imageOpacity":90,"maskOpacity":15,"surfaceOpacity":60,"blurStrength":32}`)
	require.NoError(t, err)
	assert.True(t, value.GlassEnabled)
	assert.Equal(t, 32, value.BlurStrength)
	assert.Equal(t, 60, value.SurfaceOpacity)
	for _, raw := range []string{`null`, `[]`, `{"blurStrength":61}`, `{"blurStrength":-1}`, `{"imageOpacity":101}`, `{"surfaceOpacity":-1}`} {
		_, err := ParseAppearanceSettings(raw)
		require.Error(t, err, raw)
	}
}

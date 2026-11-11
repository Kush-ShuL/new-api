package system_setting

import (
	"errors"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const AppearanceOptionKey = "AppearanceSettings"

type AppearanceSettings struct {
	GlassEnabled   bool   `json:"glassEnabled"`
	Background     string `json:"background"`
	ImageOpacity   int    `json:"imageOpacity"`
	MaskOpacity    int    `json:"maskOpacity"`
	SurfaceOpacity int    `json:"surfaceOpacity"`
	BlurStrength   int    `json:"blurStrength"`
}

var appearanceCache struct {
	sync.Mutex
	raw   string
	value AppearanceSettings
	ready bool
}

func ParseAppearanceSettings(raw string) (AppearanceSettings, error) {
	value := AppearanceSettings{ImageOpacity: 85, MaskOpacity: 35, SurfaceOpacity: 85, BlurStrength: 20}
	if raw == "" {
		return value, nil
	}
	if common.GetJsonType(common.RawMessage(raw)) != "object" || common.UnmarshalJsonStr(raw, &value) != nil ||
		len(value.Background) > 1_500_000 ||
		value.ImageOpacity < 0 || value.ImageOpacity > 100 ||
		value.MaskOpacity < 0 || value.MaskOpacity > 100 ||
		value.SurfaceOpacity < 0 || value.SurfaceOpacity > 100 ||
		value.BlurStrength < 0 || value.BlurStrength > 60 {
		return AppearanceSettings{}, errors.New("Invalid appearance settings")
	}
	return value, nil
}

func AppearanceSettingsSnapshot() AppearanceSettings {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[AppearanceOptionKey]
	common.OptionMapRWMutex.RUnlock()
	appearanceCache.Lock()
	defer appearanceCache.Unlock()
	if !appearanceCache.ready || appearanceCache.raw != raw {
		value, err := ParseAppearanceSettings(raw)
		if err != nil {
			value, _ = ParseAppearanceSettings("")
		}
		appearanceCache.raw, appearanceCache.value, appearanceCache.ready = raw, value, true
	}
	return appearanceCache.value
}

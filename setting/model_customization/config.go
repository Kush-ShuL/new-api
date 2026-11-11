package model_customization

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const OptionKey = "ModelCustomizations"

type ModelCustomization struct {
	DisplayName string `json:"display_name"`
	Icon        string `json:"icon"`
	Background  string `json:"background,omitempty"`
}

var cache struct {
	sync.Mutex
	raw    string
	models map[string]ModelCustomization
}

func Parse(raw string) (map[string]ModelCustomization, error) {
	models := map[string]ModelCustomization{}
	if raw == "" {
		return models, nil
	}
	if err := common.UnmarshalJsonStr(raw, &models); err != nil || models == nil {
		return nil, fmt.Errorf("model customizations must be a JSON object")
	}
	if len(models) > 256 {
		return nil, fmt.Errorf("at most 256 model customizations are allowed")
	}
	for name, entry := range models {
		if strings.TrimSpace(name) == "" || len(name) > 256 || len(entry.DisplayName) > 256 || len(entry.Icon) > 200_000 || len(entry.Background) > 4096 {
			return nil, fmt.Errorf("invalid model customization for %q", name)
		}
	}
	return models, nil
}

// Get returns an immutable snapshot; image data is decoded only when settings change.
func Get() map[string]ModelCustomization {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[OptionKey]
	common.OptionMapRWMutex.RUnlock()
	cache.Lock()
	defer cache.Unlock()
	if cache.models == nil || cache.raw != raw {
		models, err := Parse(raw)
		if err == nil {
			cache.raw, cache.models = raw, models
		}
	}
	return cache.models
}

package geomap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/mods/nums"
)

type Icon struct {
	Name         string    `json:"name"`
	IconUrl      string    `json:"iconUrl"`
	IconSize     []float64 `json:"iconSize,omitempty"`
	IconAnchor   []float64 `json:"iconAnchor,omitempty"`
	PopupAnchor  []float64 `json:"popupAnchor,omitempty"`
	ShadowUrl    string    `json:"shadowUrl,omitempty"`
	ShadowSize   []float64 `json:"shadowSize,omitempty"`
	ShadowAnchor []float64 `json:"shadowAnchor,omitempty"`
}

type Layer struct {
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	Jsonized string             `json:"jsonized"`
	Option   nums.GeoProperties `json:"option,omitempty"`
	Popup    *Popup             `json:"popup,omitempty"`
	Style    string             `json:"pointStyle,omitempty"`
}

type Popup struct {
	Content string `json:"content"`
	Open    bool   `json:"open,omitempty"`
}

func NewPopupMap(m map[string]interface{}) *Popup {
	if m == nil {
		return nil
	}
	if popup, ok := m["popup"]; ok {
		if popupMap, ok := popup.(map[string]any); ok {
			ret := &Popup{}
			if content, ok := popupMap["content"]; ok {
				ret.Content = content.(string)
			} else {
				return nil
			}
			if open, ok := popupMap["open"]; ok {
				if flag, ok := open.(bool); ok {
					ret.Open = flag
				}
			}
			delete(m, "popup")
			return ret
		}
	}
	return nil
}

func MarshalJS(gp map[string]any) (string, error) {
	keys := []string{}
	for k := range gp {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	fields := []string{}
	for _, k := range keys {
		vv := gp[k]
		var line string
		if k == "icon" {
			line = fmt.Sprintf("%s:%v", k, vv)
		} else {
			switch v := vv.(type) {
			case bool:
				line = fmt.Sprintf("%s:%t", k, v)
			case string:
				line = fmt.Sprintf("%s:%q", k, v)
			case map[string]any:
				if str, err := MarshalJS(v); err != nil {
					return "", err
				} else {
					line = str
				}
			default:
				line = fmt.Sprintf("%s:%v", k, v)
			}
		}
		fields = append(fields, line)
	}
	return "{" + strings.Join(fields, ",") + "}", nil
}

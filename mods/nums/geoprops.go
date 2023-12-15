package nums

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type GeoProperties map[string]any

func NewGeoPropertiesParse(opt string) (GeoProperties, error) {
	if !strings.HasPrefix(strings.TrimSpace(opt), "{") {
		opt = "{" + opt + "}"
	}
	ret := GeoProperties{}
	err := json.Unmarshal([]byte(opt), &ret)
	return ret, err
}

func (gp GeoProperties) Copy(other GeoProperties) {
	for k, v := range other {
		gp[k] = v
	}
}

func (gp GeoProperties) PopString(name string) (string, bool) {
	if v, ok := gp[name]; ok {
		delete(gp, name)
		if str, ok := v.(string); ok {
			return str, true
		} else {
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}

func (gp GeoProperties) PopBool(name string) (bool, bool) {
	if v, ok := gp[name]; ok {
		delete(gp, name)
		if b, ok := v.(bool); ok {
			return b, true
		} else if str, ok := v.(string); ok {
			if b, err := strconv.ParseBool(str); err == nil {
				return b, true
			}
		}
	}
	return false, false
}

func (gp GeoProperties) MarshalJS() (string, error) {
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
			case int:
				line = fmt.Sprintf("%s:%d", k, v)
			case float64:
				line = fmt.Sprintf("%s:%v", k, v)
			case bool:
				line = fmt.Sprintf("%s:%t", k, v)
			default:
				line = fmt.Sprintf("%s:%q", k, v)
			}
		}
		fields = append(fields, line)
	}
	return "{" + strings.Join(fields, ",") + "}", nil
}

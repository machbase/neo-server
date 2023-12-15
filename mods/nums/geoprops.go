package nums

import (
	"encoding/json"
	"fmt"
	"math"
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

type CRS struct {
	Code    string         `json:"code"`
	Proj    string         `json:"proj"`
	Options map[string]any `json:"option"`
}

// reference: https://kartena.github.io/Proj4Leaflet/
var KakaoCRS = &CRS{
	Code: "EPSG:5181",
	Proj: `+proj=tmerc +lat_0=38 +lon_0=127 +k=1 +x_0=200000 +y_0=500000 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs`,
	Options: map[string]any{
		"resolutions": []float64{2048, 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2, 1, 0.5, 0.25},
		"origin":      []float64{-30000, -60000},
		"bounds":      [][]float64{{-30000 - math.Pow(2, 19)*4, -60000}, {-30000 + math.Pow(2, 19)*5, -60000 + math.Pow(2, 19)*5}},
	},
}

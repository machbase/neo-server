package tql

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
	"github.com/paulmach/orb/geojson"
	"gonum.org/v1/gonum/stat"
)

func enableModuleRegistry(ctx *JSContext) {
	registry := require.NewRegistry(require.WithLoader(jsSourceLoad))
	registry.RegisterNativeModule("system", ctx.nativeModuleSystem)
	registry.RegisterNativeModule("generator", ctx.nativeModuleGenerator)
	registry.RegisterNativeModule("filter", ctx.nativeModuleFilter)
	registry.RegisterNativeModule("stat", ctx.nativeModuleStat)
	registry.RegisterNativeModule("dsp", ctx.nativeModuleDsp)
	registry.RegisterNativeModule("geo", ctx.nativeModuleGeo)
	registry.Enable(ctx.vm)
}

func (ctx *JSContext) nativeModuleSystem(r *js.Runtime, module *js.Object) {
	// m = require("system")
	o := module.Get("exports").(*js.Object)
	// m.free_os_memory()
	o.Set("free_os_memory", func() {
		debug.FreeOSMemory()
	})
	// m.gc()
	o.Set("gc", func() {
		runtime.GC()
	})
	// m.now()
	o.Set("now", func() js.Value {
		return ctx.vm.ToValue(time.Now())
	})
	// m.inflight()
	o.Set("inflight", func() js.Value {
		ret := ctx.vm.NewObject()
		ret.Set("set", func(name string, value js.Value) js.Value {
			if inf := ctx.node.Inflight(); inf != nil {
				inf.SetVariable(name, value.Export())
			}
			return js.Undefined()
		})
		ret.Set("get", func(name string) js.Value {
			if inf := ctx.node.Inflight(); inf != nil {
				if v, err := inf.GetVariable("$" + name); err != nil {
					return ctx.vm.NewGoError(fmt.Errorf("SCRIPT %s", err.Error()))
				} else {
					return ctx.vm.ToValue(v)
				}
			}
			return js.Undefined()
		})
		return ret
	})
	// m.publisher({bridge:"name"})
	o.Set("publisher", func(optObj map[string]any) js.Value {
		var cname string
		if len(optObj) > 0 {
			// parse db options `$.publisher({bridge: "name"})`
			if br, ok := optObj["bridge"]; ok {
				cname = br.(string)
			}
		}
		br, err := bridge.GetBridge(cname)
		if err != nil || br == nil {
			return ctx.vm.NewGoError(fmt.Errorf("publisher: bridge '%s' not found", cname))
		}

		ret := ctx.vm.NewObject()
		if mqttC, ok := br.(*bridge.MqttBridge); ok {
			ret.Set("publish", func(topic string, payload any) js.Value {
				flag, err := mqttC.Publish(topic, payload)
				if err != nil {
					return ctx.vm.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
				}
				return ctx.vm.ToValue(flag)
			})
		} else if natsC, ok := br.(*bridge.NatsBridge); ok {
			ret.Set("publish", func(subject string, payload any) js.Value {
				flag, err := natsC.Publish(subject, payload)
				if err != nil {
					return ctx.vm.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
				}
				return ctx.vm.ToValue(flag)
			})
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("publisher: bridge '%s' not supported", cname))
		}
		return ret
	})
	// m.statz("1m", ...keys)
	o.Set("statz", func(samplingInterval string, keyFilters ...string) js.Value {
		var interval = api.MetricShortTerm
		switch strings.ToLower(samplingInterval) {
		case "short":
			interval = api.MetricShortTerm
		case "mid":
			interval = api.MetricMidTerm
		case "long":
			interval = api.MetricLongTerm
		default:
			if dur, err := time.ParseDuration(samplingInterval); err == nil {
				interval = dur
			}
		}
		statz := api.QueryStatz(interval, api.QueryStatzFilter(keyFilters))
		if statz.Err != nil {
			return ctx.vm.NewGoError(statz.Err)
		}
		ret := []map[string]any{}
		for _, row := range statz.Rows {
			m := map[string]any{
				"time":   row.Timestamp,
				"values": row.Values,
			}
			for i, v := range row.Values {
				m[strings.ReplaceAll(keyFilters[i], ":", "_")] = v
			}
			ret = append(ret, m)
			//ret = append(ret, append([]any{row.Timestamp}, row.Values...))
		}
		return ctx.vm.ToValue(ret)
	})
}

func (ctx *JSContext) nativeModuleGenerator(r *js.Runtime, module *js.Object) {
	// m = require("generator")
	o := module.Get("exports").(*js.Object)
	// m.random()
	o.Set("random", func() js.Value {
		return ctx.vm.ToValue(rand.Float64())
	})
	// m.simplex(seed)
	o.Set("simplex", func(seed int64) js.Value {
		var gen *opensimplex.Generator
		ret := ctx.vm.NewObject()
		ret.Set("eval", func(dim ...float64) float64 {
			if gen == nil {
				gen = opensimplex.New(seed)
			}
			return gen.Eval(dim...)
		})
		return ret
	})
	// m.uuid(version)
	o.Set("uuid", func(version int) js.Value {
		if slices.Contains([]int{1, 4, 6, 7}, version) {
			var gen uuid.Generator
			ret := ctx.vm.NewObject()
			ret.Set("eval", func() js.Value {
				if gen == nil {
					gen = uuid.NewGen()
				}
				var uid uuid.UUID
				switch version {
				case 1:
					uid, _ = gen.NewV1()
				case 4:
					uid, _ = gen.NewV4()
				case 6:
					uid, _ = gen.NewV6()
				case 7:
					uid, _ = gen.NewV7()
				}
				return ctx.vm.ToValue(uid.String())
			})
			return ret
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("uuid: unsupported version %d", version))
		}
	})
	// m.arrange(begin, end, step)
	o.Set("arrange", func(start, stop, step float64) js.Value {
		if step == 0 {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: step must not be 0"))
		}
		if start == stop {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: start and stop must not be equal"))
		}
		if start < stop && step < 0 {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: step must be positive"))
		}
		if start > stop && step > 0 {
			return ctx.vm.NewGoError(fmt.Errorf("arrange: step must be negative"))
		}
		length := int(math.Abs((stop-start)/step)) + 1
		arr := make([]float64, length)
		for i := 0; i < length; i++ {
			arr[i] = start + float64(i)*step
		}
		return ctx.vm.ToValue(arr)
	})
	// m.linspace(begin, end, count)
	o.Set("linspace", func(start, stop float64, count int) js.Value {
		return ctx.vm.ToValue(nums.Linspace(start, stop, count))
	})
	// m.meshgrid(arr1, arr2)
	o.Set("meshgrid", func(arr1, arr2 []float64) js.Value {
		len_x := len(arr1)
		len_y := len(arr2)
		arr := make([][]float64, len_x*len_y)
		for x, v1 := range arr1 {
			for y, v2 := range arr2 {
				arr[x*len_y+y] = []float64{v1, v2}
			}
		}
		return ctx.vm.ToValue(arr)
	})
}

func (ctx *JSContext) nativeModuleFilter(r *js.Runtime, module *js.Object) {
	// m = require("filter")
	o := module.Get("exports").(*js.Object)
	// lpf = m.lowpass(alpha); newValue = lpf.eval(value);
	o.Set("lowpass", func(alpha float64) js.Value {
		if alpha <= 0 || alpha >= 1 {
			return ctx.vm.NewGoError(errors.New("alpha should be 0 < alpha < 1"))
		}
		ret := ctx.vm.NewObject()
		prev := float64(math.MaxInt64)
		ret.Set("eval", func(value float64) float64 {
			if prev == math.MaxInt64 {
				prev = value
			} else {
				prev = (1-alpha)*prev + alpha*value
			}
			return prev
		})
		return ret
	})
}

func (ctx *JSContext) nativeModuleStat(r *js.Runtime, module *js.Object) {
	// m = require("stat")
	o := module.Get("exports").(*js.Object)
	// m.mean(array)
	o.Set("mean", func(arr []float64) float64 {
		return stat.Mean(arr, nil)
	})
	// m.stdDev(array)
	o.Set("stdDev", func(arr []float64) float64 {
		return stat.StdDev(arr, nil)
	})
	// m.quantile(p, array)
	o.Set("quantile", func(p float64, arr []float64) float64 {
		slices.Sort(arr)
		return stat.Quantile(p, stat.Empirical, arr, nil)
	})
}

func (ctx *JSContext) nativeModuleDsp(r *js.Runtime, module *js.Object) {
	// m = require("dsp")
	o := module.Get("exports").(*js.Object)
	// m.fft(times, values)
	o.Set("fft", func(times []any, values []any) js.Value {
		ts := make([]time.Time, len(times))
		vs := make([]float64, len(values))
		for i, val := range times {
			switch v := val.(type) {
			case time.Time:
				ts[i] = v
			case *time.Time:
				ts[i] = *v
			default:
				return ctx.vm.NewGoError(fmt.Errorf("FFTError invalid %dth sample time, but %T", i, val))
			}
		}
		for i, val := range values {
			switch v := val.(type) {
			case float64:
				vs[i] = v
			case *float64:
				vs[i] = *v
			default:
				return ctx.vm.NewGoError(fmt.Errorf("FFTError invalid %dth sample value, but %T", i, val))
			}
		}
		xs, ys := fft.FastFourierTransform(ts, vs)
		return ctx.vm.ToValue(map[string]any{"x": xs, "y": ys})
	})
}

func (ctx *JSContext) nativeModuleGeo(r *js.Runtime, module *js.Object) {
	// m = require("geo")
	o := module.Get("exports").(*js.Object)
	o.Set("haversine", func(lat1, lon1, lat2, lon2 float64) float64 {
		// EarthRadius is the radius of the earth in meters.
		// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
		const EarthRadius = 6378137.0 // meters
		diffLat := lat2 - lat1
		diffLon := lon2 - lon1
		a := math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(diffLon/2), 2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
		return c * EarthRadius
	})
	// m.parseGeoJSON(value)
	o.Set("parseGeoJSON", func(value js.Value) js.Value {
		obj := value.ToObject(ctx.vm)
		if obj == nil {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError requires a GeoJSON object, but got %q", value.ExportType()))
		}
		typeString := obj.Get("type")
		if typeString == nil {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError missing a GeoJSON type"))
		}
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
		var geoObj any
		switch typeString.String() {
		case "FeatureCollection":
			if geo, err := geojson.UnmarshalFeatureCollection(jsonBytes); err == nil {
				geoObj = geo
			} else {
				return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
			}
		case "Feature":
			if geo, err := geojson.UnmarshalFeature(jsonBytes); err == nil {
				geoObj = geo
			} else {
				return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
			}
		case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
			if geo, err := geojson.UnmarshalGeometry(jsonBytes); err == nil {
				geoObj = geo
			} else {
				return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
			}
		default:
			return ctx.vm.NewGoError(fmt.Errorf("GeoJSONError %s", "unsupported GeoJSON type"))
		}
		var _ = geoObj
		return obj
	})
}

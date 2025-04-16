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
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/mods/nums/fft"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
	"github.com/paulmach/orb/geojson"
	"gonum.org/v1/gonum/stat"
)

func enableModuleRegistry(ctx *GojaContext) {
	registry := require.NewRegistry(require.WithLoader(jsSourceLoad))
	registry.RegisterNativeModule("system", ctx.nativeModuleSystem)
	registry.RegisterNativeModule("generator", ctx.nativeModuleGenerator)
	registry.RegisterNativeModule("filter", ctx.nativeModuleFilter)
	registry.RegisterNativeModule("stat", ctx.nativeModuleStat)
	registry.RegisterNativeModule("dsp", ctx.nativeModuleDsp)
	registry.RegisterNativeModule("geo", ctx.nativeModuleGeo)
	registry.Enable(ctx.vm)
}

func (ctx *GojaContext) nativeModuleSystem(r *goja.Runtime, module *goja.Object) {
	// m = require("system")
	o := module.Get("exports").(*goja.Object)
	// m.free_os_memory()
	o.Set("free_os_memory", func() {
		debug.FreeOSMemory()
	})
	// m.gc()
	o.Set("gc", func() {
		runtime.GC()
	})
	// m.now()
	o.Set("now", func() goja.Value {
		return ctx.vm.ToValue(time.Now())
	})
}

func (ctx *GojaContext) nativeModuleGenerator(r *goja.Runtime, module *goja.Object) {
	// m = require("generator")
	o := module.Get("exports").(*goja.Object)
	// m.random()
	o.Set("random", func() goja.Value {
		return ctx.vm.ToValue(rand.Float64())
	})
	// m.simplex(seed)
	o.Set("simplex", func(seed int64) goja.Value {
		ret := &GojaSimpleX{seed: seed}
		return ctx.vm.ToValue(ret)
	})
	// m.uuid(version)
	o.Set("uuid", func(version int) goja.Value {
		if slices.Contains([]int{1, 4}, version) {
			ret := &GojaUUID{ver: version}
			return ctx.vm.ToValue(ret)
		} else {
			return ctx.vm.NewGoError(fmt.Errorf("uuid: unsupported version %d", version))
		}
	})
}

func (ctx *GojaContext) nativeModuleFilter(r *goja.Runtime, module *goja.Object) {
	// m = require("filter")
	o := module.Get("exports").(*goja.Object)
	// lpf = m.lowpass(alpha); newValue = lpf.Eval(value);
	o.Set("lowpass", func(alpha float64) goja.Value {
		if alpha <= 0 || alpha >= 1 {
			return ctx.vm.NewGoError(errors.New("alpha should be 0 < alpha < 1 "))
		}
		lpf := &lowPassFilter{alpha: alpha, prev: math.MaxInt64}
		return ctx.vm.ToValue(lpf)
	})
}

func (lpf *lowPassFilter) Eval(value float64) float64 {
	if lpf.prev == math.MaxInt64 {
		lpf.prev = value
	} else {
		lpf.prev = (1-lpf.alpha)*lpf.prev + lpf.alpha*value
	}
	return lpf.prev
}

func (ctx *GojaContext) nativeModuleStat(r *goja.Runtime, module *goja.Object) {
	// m = require("stat")
	o := module.Get("exports").(*goja.Object)
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

func (ctx *GojaContext) nativeModuleDsp(r *goja.Runtime, module *goja.Object) {
	// m = require("dsp")
	o := module.Get("exports").(*goja.Object)
	// m.fft(times, values)
	o.Set("fft", func(times []any, values []any) goja.Value {
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

type GojaUUID struct {
	ver int
	gen uuid.Generator
}

func (u *GojaUUID) Eval() string {
	if u.gen == nil {
		u.gen = uuid.NewGen()
	}
	var uid uuid.UUID
	switch u.ver {
	case 1:
		uid, _ = u.gen.NewV1()
	case 4:
		uid, _ = u.gen.NewV4()
	case 6:
		uid, _ = u.gen.NewV6()
	case 7:
		uid, _ = u.gen.NewV7()
	}
	return uid.String()
}

type GojaSimpleX struct {
	seed int64
	gen  *opensimplex.Generator
}

func (sx *GojaSimpleX) Eval(dim ...float64) float64 {
	if sx.gen == nil {
		sx.gen = opensimplex.New(sx.seed)
	}
	return sx.gen.Eval(dim...)
}

const (
	// EarthRadius is the radius of the earth in meters.
	// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
	EarthRadius = 6378137.0 // meters
)

func (ctx *GojaContext) nativeModuleGeo(r *goja.Runtime, module *goja.Object) {
	// m = require("geo")
	o := module.Get("exports").(*goja.Object)
	o.Set("haversine", func(lat1, lon1, lat2, lon2 float64) float64 {
		diffLat := lat2 - lat1
		diffLon := lon2 - lon1
		a := math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(diffLon/2), 2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
		return c * EarthRadius
	})
	// m.parseGeoJSON(value)
	o.Set("parseGeoJSON", func(value goja.Value) goja.Value {
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

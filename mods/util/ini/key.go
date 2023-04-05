package ini

import (
	"fmt"
	"strconv"
	"strings"
)

type Key interface {
	// name of key
	Name() string
	// value of key
	Value() (string, error)
	ValueWithDefault(def string) string

	Bool() (bool, error)
	BoolWithDefault(def bool) bool

	Int() (int, error)
	IntWithDefault(def int) int

	Uint() (uint, error)
	UintWithDefault(def uint) uint

	Int64() (int64, error)
	Int64WithDefault(def int64) int64

	Float32() (float32, error)
	Float32WithDefault(def float32) float32

	Float64() (float64, error)
	Float64WithDefault(def float64) float64

	// return a string as "key=value" format
	String() string
}

type nonExistKey struct {
	Key
	name string
}

func newNonExistKey(name string) Key {
	return &nonExistKey{name: name}
}

func (non *nonExistKey) Name() string {
	return non.name
}

func (non *nonExistKey) noSuchKey() error {
	return fmt.Errorf("no such key:%s", non.name)
}

func (non *nonExistKey) Value() (string, error) {
	return "", non.noSuchKey()
}

func (non *nonExistKey) ValueWithDefault(def string) string {
	return def
}

func (non *nonExistKey) Bool() (bool, error) {
	return false, non.noSuchKey()
}

func (non *nonExistKey) BoolWithDefault(def bool) bool {
	return def
}

func (non *nonExistKey) Int() (int, error) {
	return 0, non.noSuchKey()
}

func (non *nonExistKey) IntWithDefault(def int) int {
	return def
}

func (non *nonExistKey) Uint() (uint, error) {
	return 0, non.noSuchKey()
}

func (non *nonExistKey) UintWithDefault(def uint) uint {
	return def
}

func (non *nonExistKey) Int64() (int64, error) {
	return 0, non.noSuchKey()
}

func (non *nonExistKey) Int64WithDefault(def int64) int64 {
	return def
}

func (non *nonExistKey) Float32() (float32, error) {
	return 0, non.noSuchKey()
}

func (non *nonExistKey) Float32WithDefault(def float32) float32 {
	return def
}

func (non *nonExistKey) Float64() (float64, error) {
	return 0, non.noSuchKey()
}

func (non *nonExistKey) Float64WithDefault(def float64) float64 {
	return def
}

func (non *nonExistKey) String() string {
	return ""
}

type normalKey struct {
	name  string
	value string
}

func newNormalKey(name string, value string) Key {
	return &normalKey{name: name, value: value}
}

func (k *normalKey) Name() string {
	return k.name
}

func (k *normalKey) Value() (string, error) {
	return k.value, nil
}

func (k *normalKey) ValueWithDefault(def string) string {
	return k.value
}

var trueBoolValue = map[string]bool{"true": true, "t": true, "yes": true, "y": true, "1": true, "on": true}

func (k *normalKey) Bool() (bool, error) {
	if _, ok := trueBoolValue[strings.ToLower(k.value)]; ok {
		return true, nil
	}
	return false, nil
}

func (k *normalKey) BoolWithDefault(def bool) bool {
	if v, err := k.Bool(); err != nil {
		return def
	} else {
		return v
	}
}

func (k *normalKey) Int() (int, error) {
	return strconv.Atoi(k.value)
}

func (k *normalKey) IntWithDefault(def int) int {
	if v, err := k.Int(); err != nil {
		return def
	} else {
		return v
	}
}

func (k *normalKey) Uint() (uint, error) {
	v, err := strconv.ParseUint(k.value, 0, 32)
	return uint(v), err
}

func (k *normalKey) UintWithDefault(def uint) uint {
	if v, err := k.Uint(); err != nil {
		return def
	} else {
		return v
	}
}

func (k *normalKey) Int64() (int64, error) {
	return strconv.ParseInt(k.value, 0, 64)
}

func (k *normalKey) Int64WithDefault(def int64) int64 {
	if v, err := k.Int64(); err != nil {
		return def
	} else {
		return v
	}
}

func (k *normalKey) Float32() (float32, error) {
	v, err := strconv.ParseFloat(k.value, 32)
	return float32(v), err
}

func (k *normalKey) Float32WithDefault(def float32) float32 {
	if v, err := k.Float32(); err != nil {
		return def
	} else {
		return v
	}
}

func (k *normalKey) Float64() (float64, error) {
	return strconv.ParseFloat(k.value, 64)
}

func (k *normalKey) Float64WithDefault(def float64) float64 {
	if v, err := k.Float64(); err != nil {
		return def
	} else {
		return v
	}
}

func (k *normalKey) String() string {
	return fmt.Sprintf("%s=%v", k.name, toEscape(k.value))
}

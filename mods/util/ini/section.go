package ini

import (
	"bytes"
	"fmt"
	"io"
)

type Section struct {
	Name string
	keys map[string]Key
}

func NewSection(name string) *Section {
	return &Section{
		Name: name,
		keys: make(map[string]Key),
	}
}

func (section *Section) Add(key, value string) {
	section.keys[key] = newNormalKey(key, value)
}

func (section *Section) HasKey(key string) bool {
	_, ok := section.keys[key]
	return ok
}

func (section *Section) Keys() []Key {
	r := make([]Key, 0)
	for _, v := range section.keys {
		r = append(r, v)
	}
	return r
}

func (section *Section) Key(key string) Key {
	if v, ok := section.keys[key]; ok {
		return v
	}
	return newNonExistKey(key)
}

func (section *Section) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	section.Write(buf)
	return buf.String()
}

func (section *Section) Write(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "[%s]\n", section.Name); err != nil {
		return err
	}

	for _, v := range section.keys {
		if _, err := fmt.Fprintf(writer, "%s\n", v.String()); err != nil {
			return err
		}
	}
	return nil
}

func (section *Section) GetValue(key string) (string, error) {
	return section.Key(key).Value()
}

func (section *Section) GetValueWithDefault(key string, def string) string {
	return section.Key(key).ValueWithDefault(def)
}

func (section *Section) GetBool(key string) (bool, error) {
	return section.Key(key).Bool()
}

func (section *Section) GetBoolWithDefault(key string, def bool) bool {
	return section.Key(key).BoolWithDefault(def)
}

func (section *Section) GetInt(key string) (int, error) {
	return section.Key(key).Int()
}

func (section *Section) GetIntWithDefault(key string, def int) int {
	return section.Key(key).IntWithDefault(def)
}

func (section *Section) GetUint(key string) (uint, error) {
	return section.Key(key).Uint()
}

func (section *Section) GetUintWithDefault(key string, def uint) uint {
	return section.Key(key).UintWithDefault(def)
}

func (section *Section) GetInt64(key string) (int64, error) {
	return section.Key(key).Int64()
}

func (section *Section) GetInt64WithDefault(key string, def int64) int64 {
	return section.Key(key).Int64WithDefault(def)
}

func (section *Section) GetFloat32(key string) (float32, error) {
	return section.Key(key).Float32()
}

func (section *Section) GetFloat32WithDefault(key string, def float32) float32 {
	return section.Key(key).Float32WithDefault(def)
}

func (section *Section) GetFloat64(key string) (float64, error) {
	return section.Key(key).Float64()
}

func (section *Section) GetFloat64WithDefault(key string, def float64) float64 {
	return section.Key(key).Float64WithDefault(def)
}

package ini

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

type Ini struct {
	defaultSectionName string
	sections           map[string]*Section
}

/*
Load the .ini from one of following resource:
  - file
  - string in .ini format
  - byte array in .ini format
  - io.Reader a reader to load .ini content

One or more source can be provided in this Load method, such as:

	var reader1 io.Reader = ...
	var reader2 io.Reader = ...
	ini.Load( "./my.ini", "[section]\nkey=1", "./my2.ini", reader1, reader2 )
*/
func Load(sources ...any) *Ini {
	ini := New()
	ini.SetDefaultSectionName("default")
	ini.Load(sources...)
	return ini
}

func New() *Ini {
	return &Ini{
		defaultSectionName: "default",
		sections:           make(map[string]*Section),
	}
}

func (ini *Ini) DefaultSectionName() string {
	return ini.defaultSectionName
}

func (ini *Ini) SetDefaultSectionName(defName string) {
	ini.defaultSectionName = defName
}

// create a new section if the section with name does not exist
// or return the exist one if the section with name already exists
func (ini *Ini) NewSection(name string) *Section {
	if section, ok := ini.sections[name]; ok {
		return section
	}
	section := NewSection(name)
	ini.sections[name] = section
	return section
}

// add a section to the .ini file and overwrite the exist section
// with same name
func (ini *Ini) AddSection(section *Section) {
	ini.sections[section.Name] = section
}

func (ini *Ini) Sections() []*Section {
	r := make([]*Section, 0)
	for _, section := range ini.sections {
		r = append(r, section)
	}
	return r
}

func (ini *Ini) Section(sectionName string) (*Section, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section, nil
	}
	return nil, ini.noSuchSection(sectionName)
}

func (ini *Ini) HasSection(sectionName string) bool {
	_, ok := ini.sections[sectionName]
	return ok
}

func (ini *Ini) HasKey(sectionName, key string) bool {
	if section, ok := ini.sections[sectionName]; ok {
		return section.HasKey(key)
	}
	return false
}

func (ini *Ini) GetValue(sectionName, key string) (string, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetValue(key)
	}
	return "", ini.noSuchSection(sectionName)
}

func (ini *Ini) GetValueWithDefault(sectionName, key string, def string) string {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetValueWithDefault(key, def)
	}
	return def
}

func (ini *Ini) GetBool(sectionName, key string) (bool, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetBool(key)
	}
	return false, ini.noSuchSection(sectionName)
}

func (ini *Ini) GetBoolWithDefault(sectionName, key string, def bool) bool {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetBoolWithDefault(key, def)
	}
	return def
}

func (ini *Ini) GetInt(sectionName, key string) (int, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetInt(key)
	}
	return 0, ini.noSuchSection(sectionName)
}

func (ini *Ini) GetIntWithDefault(sectionName, key string, def int) int {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetIntWithDefault(key, def)
	}
	return def
}

func (ini *Ini) GetUint(sectionName, key string) (uint, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetUint(key)
	}
	return 0, ini.noSuchSection(sectionName)
}

func (ini *Ini) GetUintWithDefault(sectionName, key string, def uint) uint {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetUintWithDefault(key, def)
	}
	return def
}

func (ini *Ini) GetInt64(sectionName, key string) (int64, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetInt64(key)
	}
	return 0, ini.noSuchSection(sectionName)
}

func (ini *Ini) GetInt64WithDefault(sectionName, key string, def int64) int64 {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetInt64WithDefault(key, def)
	}
	return def
}

func (ini *Ini) GetFloat32(sectionName, key string) (float32, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetFloat32(key)
	}
	return 0, ini.noSuchSection(sectionName)
}

func (ini *Ini) GetFloat32WithDefault(sectionName, key string, def float32) float32 {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetFloat32WithDefault(key, def)
	}
	return def
}

func (ini *Ini) GetFloat64(sectionName, key string) (float64, error) {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetFloat64(key)
	}
	return 0, ini.noSuchSection(sectionName)
}

func (ini *Ini) GetFloat64WithDefault(sectionName, key string, def float64) float64 {
	if section, ok := ini.sections[sectionName]; ok {
		return section.GetFloat64WithDefault(key, def)
	}
	return def
}

func (ini *Ini) noSuchSection(sectionName string) error {
	return fmt.Errorf("no such section:%s", sectionName)
}

func (ini *Ini) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	ini.Write(buf)
	return buf.String()
}

func (ini *Ini) Write(writer io.Writer) error {
	for _, section := range ini.sections {
		err := section.Write(writer)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ini *Ini) WriteToFile(filename string) error {
	file, err := os.Create(filename)
	if err == nil {
		defer file.Close()
		return ini.Write(file)
	}
	return err
}

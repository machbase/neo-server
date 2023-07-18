package client

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/ini"
)

type Pref struct {
	store     *ini.Ini
	storePath string
}

func PrefDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	prefDir := filepath.Join(homeDir, ".config", "machbase", "neoshell")
	return prefDir
}

func LoadPref() (*Pref, error) {
	prefDir := PrefDir()
	if err := util.MkDirIfNotExists(prefDir); err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	pref := &Pref{
		store:     ini.New(),
		storePath: filepath.Join(prefDir, "neoshell.ini"),
	}

	_, err := os.Stat(pref.storePath)
	if err != nil && os.IsNotExist(err) {
	} else if err != nil {
		return nil, err
	} else {
		if err := pref.store.LoadFile(pref.storePath); err != nil {
			return nil, err
		}
	}

	return pref, nil
}

func (p *Pref) Save() error {
	return p.store.WriteToFile(p.storePath)
}

func (p *Pref) SaveTo(w io.Writer) error {
	return p.store.Write(w)
}

type PrefItem struct {
	Section string
	Name    string
	Default any
	Enum    []string

	desc     string
	validate func(string) (string, bool)
	pref     *Pref
}

func (pi *PrefItem) DefaultString() string {
	switch v := (any)(pi.Default).(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case *time.Location:
		return v.String()
	case func() string:
		return v()
	}
	return "<undefined>"
}

var prefTrueValues = []string{"true", "1", "on", "yes"}

func (pi *PrefItem) Value() string {
	if pi.pref == nil {
		return pi.DefaultString()
	}
	s := pi.pref.store.NewSection(pi.Section)
	v := s.GetValueWithDefault(pi.Name, pi.DefaultString())
	return v
}

func (pi *PrefItem) BoolValue() bool {
	v := pi.Value()
	v = strings.ToLower(v)
	for _, x := range prefTrueValues {
		if v == x {
			return true
		}
	}
	return false
}

func (pi *PrefItem) IntValue() (int, error) {
	v := pi.Value()
	i, e := strconv.ParseInt(v, 10, 64)
	return int(i), e
}

func (pi *PrefItem) TimezoneValue() *time.Location {
	v := pi.Value()
	if tz, e := time.LoadLocation(v); e != nil {
		fmt.Println(e.Error())
		return time.UTC
	} else {
		return tz
	}
}

func (pi *PrefItem) SetValue(val string) error {
	if pi.pref == nil {
		return errors.New("invalid pref store")
	}
	valid := false
	var err error
	if len(pi.Enum) == 0 {
		valid = true
	} else {
		for _, e := range pi.Enum {
			if val == e {
				valid = true
				break
			}
		}
		if !valid {
			err = fmt.Errorf("the value of '%s' should be one of %+v", pi.Name, pi.Enum)
		}
	}
	if pi.validate != nil {
		if str, ok := pi.validate(val); !ok {
			err = fmt.Errorf("invalid value of '%s'", pi.Name)
		} else {
			val = str
		}
	}
	if err != nil {
		return err
	}
	s := pi.pref.store.NewSection(pi.Section)
	s.Add(pi.Name, val)
	return nil
}

func (pi *PrefItem) Description() string {
	if len(pi.Enum) > 0 {
		return pi.desc + " [" + strings.Join(pi.Enum, ",") + "]"
	} else {
		return pi.desc
	}
}

var (
	prefItem_BoxStyle   = PrefItem{"General", "box-style", "light", []string{"simple", "bold", "double", "light", "round"}, "box style", nil, nil}
	prefItem_ViMode     = PrefItem{"General", "vi-mode", "off", []string{"on", "off"}, "use vi mode", nil, nil}
	prefItem_TimeZone   = PrefItem{"General", "tz", "Local", []string{}, "'help tz'", timezoneValidate, nil}
	prefItem_Timeformat = PrefItem{"General", "timeformat", "2006-01-02 15:04:05.999", []string{}, "'help timeformat'", timeformatValidate, nil}
	prefItem_Heading    = PrefItem{"General", "heading", "on", []string{"on", "off"}, "show heading", nil, nil}
	prefItem_Format     = PrefItem{"General", "format", "-", []string{"-", "json", "csv"}, "in/output format", nil, nil}
	prefItem_Server     = PrefItem{"General", "server", defaultServerAddress, []string{}, "default server address", nil, nil}
	prefItem_ServerCert = PrefItem{"General", "server-cert", defaultServerCertPath, []string{}, "default server cert path", nil, nil}
	prefItem_ClientCert = PrefItem{"General", "client-cert", defaultClientCertPath, []string{}, "default client cert path", nil, nil}
	prefItem_ClientKey  = PrefItem{"General", "client-key", defaultClientKeyPath, []string{}, "default client key path", nil, nil}
)

func defaultServerAddress() string {
	serverAddr := "tcp://127.0.0.1:5655"
	// check local mach_grpc.sock file
	if exePath, err := os.Executable(); err == nil {
		exeDirPath := filepath.Dir(exePath)
		sockPath := filepath.Join(exeDirPath, "mach-grpc.sock")
		if stat, err := os.Stat(sockPath); err == nil && stat != nil && !stat.IsDir() {
			serverAddr = "unix://" + sockPath
		}
	}
	return serverAddr
}

func defaultServerCertPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("ERR", err.Error())
		homeDir = "."
	}
	path := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_cert.pem")
	if nfo, err := os.Stat(path); err != nil {
		return ""
	} else {
		if nfo.IsDir() {
			return ""
		}
		return path
	}
}

func defaultClientCertPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("ERR", err.Error())
		homeDir = "."
	}
	path := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_cert.pem")
	if nfo, err := os.Stat(path); err != nil {
		return ""
	} else {
		if nfo.IsDir() {
			return ""
		}
		return path
	}
}

func defaultClientKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("ERR", err.Error())
		homeDir = "."
	}
	path := filepath.Join(homeDir, ".config", "machbase", "cert", "machbase_key.pem")
	if nfo, err := os.Stat(path); err != nil {
		return ""
	} else {
		if nfo.IsDir() {
			return ""
		}
		return path
	}
}

var prefItems = map[string]*PrefItem{
	prefItem_BoxStyle.Name:   &prefItem_BoxStyle,
	prefItem_ViMode.Name:     &prefItem_ViMode,
	prefItem_TimeZone.Name:   &prefItem_TimeZone,
	prefItem_Timeformat.Name: &prefItem_Timeformat,
	prefItem_Server.Name:     &prefItem_Server,
	prefItem_ServerCert.Name: &prefItem_ServerCert,
	prefItem_ClientCert.Name: &prefItem_ClientCert,
	prefItem_ClientKey.Name:  &prefItem_ClientKey,
	// prefItem_Heading.Name:    &prefItem_Heading,
	// prefItem_Format.Name:     &prefItem_Format,
}

func (p *Pref) Item(name string) *PrefItem {
	val, ok := prefItems[name]
	if !ok || val == nil {
		return nil
	}
	itm := &PrefItem{}
	*itm = *val
	itm.pref = p
	return itm
}

func (p *Pref) Items() []*PrefItem {
	lst := make([]*PrefItem, len(prefItems))
	idx := 0
	for _, itm := range prefItems {
		lst[idx] = &PrefItem{}
		*lst[idx] = *itm
		lst[idx].pref = p
		idx++
	}
	sort.Slice(lst, func(i, j int) bool {
		return lst[i].Name < lst[j].Name
	})
	return lst
}

func (p *Pref) BoxStyle() *PrefItem {
	return p.Item(prefItem_BoxStyle.Name)
}

func (p *Pref) ViMode() *PrefItem {
	return p.Item(prefItem_ViMode.Name)
}

func (p *Pref) TimeZone() *PrefItem {
	return p.Item(prefItem_TimeZone.Name)
}

func (p *Pref) Timeformat() *PrefItem {
	return p.Item(prefItem_Timeformat.Name)
}

func (p *Pref) Heading() *PrefItem {
	return p.Item(prefItem_Heading.Name)
}

func (p *Pref) Format() *PrefItem {
	return p.Item(prefItem_Format.Name)
}

func (p *Pref) Server() *PrefItem {
	return p.Item(prefItem_Server.Name)
}

func (p *Pref) ServerCert() *PrefItem {
	return p.Item(prefItem_ServerCert.Name)
}

func (p *Pref) ClientCert() *PrefItem {
	return p.Item(prefItem_ClientCert.Name)
}

func (p *Pref) ClientKey() *PrefItem {
	return p.Item(prefItem_ClientKey.Name)
}

func timezoneValidate(s string) (string, bool) {
	switch s {
	case "utc":
		s = "UTC"
	case "local":
		s = "Local"
	case "gmt":
		s = "GMT"
	}
	tz, err := time.LoadLocation(s)
	if err == nil {
		return tz.String(), true
	}
	tz, err = util.GetTimeLocation(s)
	if err == nil {
		return tz.String(), true
	}

	fmt.Println(err.Error())
	return "", false
}

func timeformatValidate(s string) (string, bool) {
	switch s {
	case "ns":
	case "us":
	case "ms":
	case "s":
	default:
		s = util.GetTimeformat(s)
	}
	return s, true
}

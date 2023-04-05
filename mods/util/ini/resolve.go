package ini

import (
	"bytes"
	"os"
	"strings"
)

func (ini *Ini) Resolve(predefs map[string]string, fallbackEnvVars bool) {
	for _, sect := range ini.sections {
		for k, v := range sect.keys {
			str, _ := v.Value()
			strResolved, resolved := ini.replace(str, predefs, fallbackEnvVars)
			if resolved {
				sect.Add(k, strResolved)
			}
		}
	}
}

func (ini *Ini) replace(s string, predefs map[string]string, fallbackEnvVars bool) (string, bool) {
	if !strings.Contains(s, "${") {
		return s, false
	}
	n := len(s)
	env_start_pos := -1
	resolved := false
	result := bytes.NewBuffer(make([]byte, 0))
	for i := 0; i < n; i++ {
		if env_start_pos >= 0 && s[i] != '}' {
			continue
		}
		switch s[i] {
		case '\\':
			result.WriteByte(s[i])
			if i+1 < n {
				i++
				result.WriteByte(s[i])
			}
		case '$':
			if i+1 < n && s[i+1] == '{' {
				env_start_pos = i
			} else {
				result.WriteByte(s[i])
			}
		case '}':
			if env_start_pos >= 0 {
				env_resolved := false
				env_name := s[env_start_pos+2 : i]
				dotPos := strings.Index(env_name, ".")
				if dotPos > -1 && dotPos < len(env_name)-1 {
					sectname := env_name[0:dotPos]
					keyname := env_name[dotPos+1:]
					if ini.HasKey(sectname, keyname) {
						newValue := ini.GetValueWithDefault(sectname, keyname, "")
						newValue, _ = ini.replace(newValue, predefs, fallbackEnvVars)
						result.WriteString(newValue)
						env_resolved = true
					}
				}

				if !env_resolved {
					if newValue, ok := predefs[env_name]; ok {
						newValue, _ = ini.replace(newValue, predefs, fallbackEnvVars)
						result.WriteString(newValue)
						env_resolved = true
					}
				}
				if !env_resolved && fallbackEnvVars {
					newValue := os.Getenv(env_name)
					if len(newValue) > 0 {
						newValue, _ = ini.replace(newValue, predefs, fallbackEnvVars)
						result.WriteString(newValue)
						env_resolved = true
					}
				}
				if !env_resolved {
					result.WriteString("${" + env_name + "}")
				}

				resolved = env_resolved
				env_start_pos = -1
			} else {
				result.WriteByte(s[i])
			}
		default:
			result.WriteByte(s[i])
		}
	}
	return result.String(), resolved
}

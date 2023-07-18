package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/service/httpd"
	"github.com/machbase/neo-server/mods/service/sshd"
	"github.com/machbase/neo-server/mods/util"
	"github.com/pkg/errors"
)

var reservedShellNames = []string{"SQL", "TQL", "WORKSHEET", "TAG ANALYZER", "SHELL",
	/*and more for future uses*/ "WORKBOOK", "SCRIPT", "RUN", "CMD", "COMMAND", "CONSOLE",
	/*and more for future uses*/ "MONITOR", "CHART", "DASHBOARD", "LOG", "HOME", "PLAYGROUND"}

var reservedWebShellDef = map[string]*model.ShellDefinition{
	"SQL": {Type: "sql", Label: "SQL", Icon: "file-document-outline", Id: "SQL"},
	"TQL": {Type: "tql", Label: "TQL", Icon: "chart-scatter-plot", Id: "TQL"},
	"WRK": {Type: "wrk", Label: "WORKSHEET", Icon: "clipboard-text-play-outline", Id: "WRK"},
	"TAZ": {Type: "taz", Label: "TAG ANALYZER", Icon: "chart-line", Id: "TAZ"},
	"SHELL": {Type: "term", Label: "SHELL", Icon: "console", Id: "SHELL",
		Attributes: &model.ShellAttributes{Cloneable: true},
	},
}

func (s *svr) IterateShellDefs(cb func(*model.ShellDefinition) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.shellDefsDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.shellDefsDir, entry.Name()))
		if err != nil {
			s.log.Errorf("ERR file access, %s", err.Error())
			continue
		}
		def := &model.ShellDefinition{}
		if err := json.Unmarshal(content, def); err != nil {
			s.log.Warnf("ERR invalid shell conf, %s", err.Error())
			continue
		}
		def.Id = strings.ToUpper(strings.TrimSuffix(entry.Name(), ".json"))
		// compatibility old version
		if def.Type == "" {
			def.Type = "term"
		}
		if def.Icon == "" {
			def.Icon = "console-network-outline"
		}
		if def.Label == "" {
			def.Label = def.Id
		}
		if def.Attributes == nil {
			def.Attributes = &model.ShellAttributes{
				Cloneable: true, Removable: true, Editable: true,
			}
		}
		shouldContinue := cb(def)
		if !shouldContinue {
			break
		}
	}
	return nil
}

func (s *svr) GetShellDef(id string) (found *model.ShellDefinition, err error) {
	id = strings.ToUpper(id)
	s.IterateShellDefs(func(def *model.ShellDefinition) bool {
		if def.Id == id {
			found = def
			return false
		}
		return true
	})
	return
}

func (s *svr) SetShellDef(def *model.ShellDefinition) error {
	id := strings.ToUpper(def.Id)
	for _, n := range reservedShellNames {
		if id == n {
			return fmt.Errorf("'%s' is not allowed for the custom shell name", id)
		}
	}
	if len(def.Command) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	args := util.SplitFields(def.Command, true)
	if len(args) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	binpath := args[0]
	if fi, err := os.Stat(binpath); err != nil {
		return errors.Wrapf(err, "'%s' is not accessible", binpath)
	} else {
		if fi.IsDir() {
			return fmt.Errorf("'%s' is not executable", binpath)
		}
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(strings.ToLower(binpath), ".exe") && !strings.HasSuffix(strings.ToLower(binpath), ".com") {
				return fmt.Errorf("'%s' is not executable", binpath)
			}
		} else {
			if fi.Mode().Perm()&0111 == 0 {
				return fmt.Errorf("'%s' is not executable", binpath)
			}
		}
	}
	content, err := json.Marshal(def)
	if err != nil {
		return err
	}
	path := filepath.Join(s.shellDefsDir, fmt.Sprintf("%s.json", id))
	return os.WriteFile(path, content, 0600)
}

func (s *svr) RemoveShellDef(name string) error {
	path := filepath.Join(s.shellDefsDir, fmt.Sprintf("%s.json", strings.ToUpper(name)))
	return os.Remove(path)
}

func (s *svr) RenameShellDef(name string, newName string) error {
	oldPath := filepath.Join(s.shellDefsDir, fmt.Sprintf("%s.json", strings.ToUpper(name)))
	newPath := filepath.Join(s.shellDefsDir, fmt.Sprintf("%s.json", strings.ToUpper(newName)))
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("'%s' already exists", newName)
	}
	return os.Rename(oldPath, newPath)
}

// sshd shell provider
func (s *svr) GetSshShell(id string) (found *sshd.Shell) {
	id = strings.ToUpper(id)
	s.IterateShellDefs(func(def *model.ShellDefinition) bool {
		if def.Id == id {
			found = sshShellFrom(def)
			if found != nil {
				return false
			}
		}
		return true
	})
	return
}

func (s *svr) GetAllWebShells() []*model.ShellDefinition {
	var ret []*model.ShellDefinition
	ret = append(ret, reservedWebShellDef["SQL"])
	ret = append(ret, reservedWebShellDef["TQL"])
	ret = append(ret, reservedWebShellDef["WRK"])
	ret = append(ret, reservedWebShellDef["TAZ"])
	ret = append(ret, reservedWebShellDef["SHELL"])
	s.IterateShellDefs(func(def *model.ShellDefinition) bool {
		ret = append(ret, def)
		return true
	})
	return ret
}

func (s *svr) GetWebShell(id string) (*model.ShellDefinition, error) {
	id = strings.ToUpper(id)
	ret := reservedWebShellDef[id]
	if ret != nil {
		return ret, nil
	}
	s.IterateShellDefs(func(sd *model.ShellDefinition) bool {
		if strings.ToUpper(sd.Id) == id {
			ret = sd
			return false
		}
		return true
	})
	return ret, nil
}

func (s *svr) CopyWebShell(id string) (*model.ShellDefinition, error) {
	id = strings.ToUpper(id)
	var ret *model.ShellDefinition
	if _, ok := reservedWebShellDef[id]; ok {
		ret = &model.ShellDefinition{}
		if exename, err := os.Executable(); err != nil {
			ret.Command = fmt.Sprintf(`"%s" shell`, os.Args[0])
		} else {
			ret.Command = fmt.Sprintf(`"%s" shell`, exename)
		}
	} else {
		d, err := s.GetShellDef(id)
		if err != nil {
			return nil, err
		}
		ret = d.Clone()
	}
	if ret == nil {
		s.log.Warnf("shell def not found '%s'", id)
		return nil, fmt.Errorf("shell definition not found '%s'", id)
	}
	uid, err := uuid.DefaultGenerator.NewV4()
	if err != nil {
		s.log.Warnf("shell def new id, %s", err.Error())
		return nil, err
	}
	ret.Id = uid.String()
	ret.Label = "CUSTOM SHELL"
	if err := s.SetShellDef(ret); err != nil {
		s.log.Warnf("shell def not saved", err.Error())
		return nil, err
	}
	return ret, nil
}

func (s *svr) RemoveWebShell(id string) error {
	return s.RemoveShellDef(id)
}

func (s *svr) UpdateWebShell(def *model.ShellDefinition) error {
	if err := s.SetShellDef(def); err != nil {
		s.log.Warnf("shell def not saved, %s", err.Error())
		return err
	}
	return nil
}

func sshShellFrom(def *model.ShellDefinition) *sshd.Shell {
	shell := &sshd.Shell{}
	args := util.SplitFields(def.Command, true)
	if len(args) == 0 {
		return nil
	}

	shell.Cmd = args[0]
	if len(args) > 1 {
		shell.Args = args[1:]
	}

	shell.Envs = map[string]string{}
	if runtime.GOOS == "windows" {
		envs := os.Environ()
		for _, line := range envs {
			if !strings.Contains(line, "=") {
				continue
			}
			toks := strings.SplitN(line, "=", 2)
			if len(toks) != 2 {
				continue
			}
			shell.Envs[strings.TrimSpace(toks[0])] = strings.TrimSpace(toks[1])
		}
		if _, ok := shell.Envs["USERPROFILE"]; !ok {
			userHomeDir, err := os.UserHomeDir()
			if err != nil {
				userHomeDir = "."
			}
			shell.Envs["USERPROFILE"] = userHomeDir
		}
	}
	return shell
}

func (s *svr) WebReferences() []httpd.WebReferenceGroup {
	ret := []httpd.WebReferenceGroup{}

	references := httpd.WebReferenceGroup{Label: "References"}
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://neo.machbase.com/", Target: "_blank"})
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "http://endoc.machbase.com/", Target: "_blank"})
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_blank"})
	ret = append(ret, references)

	tutorials := httpd.WebReferenceGroup{Label: "Tutorials"}
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "Waves in TQL", Addr: "./tutorials/waves_in_tql.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "Fast Fourier Transform in TQL", Addr: "./tutorials/fft_in_tql.wrk"})
	ret = append(ret, tutorials)

	samples := httpd.WebReferenceGroup{Label: "Samples"}
	samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "wrk", Title: "markdown cheatsheet", Addr: "./tutorials/sample_markdown.wrk"})
	samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "wrk", Title: "mermaid cheatsheet", Addr: "./tutorials/sample_mermaid.wrk"})
	samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "wrk", Title: "pikchr cheatsheet", Addr: "./tutorials/sample_pikchr.wrk"})
	samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "tql", Title: "user script in tql (1)", Addr: "./tutorials/user-script1.tql"})
	samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "tql", Title: "user script in tql (2)", Addr: "./tutorials/user-script2.tql"})
	ret = append(ret, samples)

	return ret
}

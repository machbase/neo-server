package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/machbase/neo-server/mods/service/httpd"
	"github.com/machbase/neo-server/mods/service/sshd"
	"github.com/pkg/errors"
)

var reservedShellNames = []string{"SQL", "TQL", "WORKSHEET", "TAG ANALYZER", "SHELL",
	/*and more for future uses*/ "WORKBOOK", "SCRIPT", "RUN", "CMD", "COMMAND", "CONSOLE",
	/*and more for future uses*/ "MONITOR", "CHART", "DASHBOARD", "LOG", "HOME", "PLAYGROUND"}

var reservedShellDef = map[string]*httpd.WebShell{
	"SQL": {Type: "sql", Label: "SQL", Icon: "file-document-outline", Id: "SQL"},
	"TQL": {Type: "tql", Label: "TQL", Icon: "chart-scatter-plot", Id: "TQL"},
	"WRK": {Type: "wrk", Label: "WORKSHEET", Icon: "clipboard-text-play-outline", Id: "WRK"},
	"TAZ": {Type: "taz", Label: "TAG ANALYZER", Icon: "chart-line", Id: "TAZ"},
	"SHELL": {Type: "term", Label: "SHELL", Icon: "console", Id: "SHELL",
		Attributes: []httpd.WebShellAttribute{&httpd.WebShellCloneable{Cloneable: true}},
	},
}

func (s *svr) IterateShellDefs(cb func(*sshd.ShellDefinition) bool) error {
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
		def := &sshd.ShellDefinition{}
		if err := json.Unmarshal(content, def); err != nil {
			s.log.Warnf("ERR invalid shell conf, %s", err.Error())
			continue
		}
		def.Name = strings.ToUpper(strings.TrimSuffix(entry.Name(), ".json"))
		shouldContinue := cb(def)
		if !shouldContinue {
			break
		}
	}
	return nil
}

func (s *svr) SetShellDef(def *sshd.ShellDefinition) error {
	name := strings.ToUpper(def.Name)
	for _, n := range reservedShellNames {
		if name == n {
			return fmt.Errorf("'%s' is not allowed for the custom shell name", name)
		}
	}
	if len(def.Args) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	binpath := def.Args[0]
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
	path := filepath.Join(s.shellDefsDir, fmt.Sprintf("%s.json", name))
	return os.WriteFile(path, content, 0600)
}

func (s *svr) RemoveShellDef(name string) error {
	path := filepath.Join(s.shellDefsDir, fmt.Sprintf("%s.json", strings.ToUpper(name)))
	return os.Remove(path)
}

// sshd shell provider
func (s *svr) GetSshShell(name string) (found *sshd.Shell) {
	name = strings.ToUpper(name)
	s.IterateShellDefs(func(def *sshd.ShellDefinition) bool {
		if def.Name == name {
			found = sshShellFrom(def)
			if found != nil {
				return false
			}
		}
		return true
	})
	return
}

func (s *svr) GetAllWebShells() []*httpd.WebShell {
	var ret []*httpd.WebShell
	ret = append(ret, reservedShellDef["SQL"])
	ret = append(ret, reservedShellDef["TQL"])
	ret = append(ret, reservedShellDef["WRK"])
	ret = append(ret, reservedShellDef["TAZ"])
	ret = append(ret, reservedShellDef["SHELL"])
	s.IterateShellDefs(func(def *sshd.ShellDefinition) bool {
		ret = append(ret, webShellFrom(def))
		return true
	})
	return ret
}

func (s *svr) GetWebShell(id string) (*httpd.WebShell, error) {
	id = strings.ToUpper(id)
	ret := reservedShellDef[id]
	if ret != nil {
		return ret, nil
	}
	s.IterateShellDefs(func(sd *sshd.ShellDefinition) bool {
		if strings.ToUpper(sd.Name) == id {
			ret = webShellFrom(sd)
			return false
		}
		return true
	})
	return ret, nil
}

func (s *svr) CopyWebShell(id string) (*httpd.WebShell, error) {
	fmt.Println("------->", "CopyWebShell", id)
	return nil, nil
}

func (s *svr) RemoveWebShell(id string) error {
	fmt.Println("------->", "RemoveWebShell", id)
	return nil

}

func (s *svr) UpdateWebShell(sh *httpd.WebShell) error {
	fmt.Println("------->", "UpdateWebShell", sh)
	return nil
}

func sshShellFrom(def *sshd.ShellDefinition) *sshd.Shell {
	shell := &sshd.Shell{}
	shell.Cmd = def.Args[0]
	if len(def.Args) > 1 {
		shell.Args = def.Args[1:]
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

func webShellFrom(def *sshd.ShellDefinition) *httpd.WebShell {
	return &httpd.WebShell{
		Type:    "term",
		Label:   def.Name,
		Icon:    "console-network-outline",
		Id:      strings.ToUpper(def.Name),
		Content: strings.Join(def.Args, " "),
		Attributes: []httpd.WebShellAttribute{
			&httpd.WebShellCloneable{Cloneable: true},
			&httpd.WebShellRemovable{Removable: true},
			&httpd.WebShellEditable{Editable: true},
		},
	}
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

package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/service/httpd"
	"github.com/machbase/neo-server/mods/service/sshd"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/ssfs"
	"github.com/pkg/errors"
)

const (
	SHELLID_SQL   = "SQL"
	SHELLID_TQL   = "TQL"
	SHELLID_WRK   = "WRK"
	SHELLID_TAZ   = "TAZ"
	SHELLID_SHELL = "SHELL"
)

const (
	SHELLTYPE_TERM = "term"
)

var reservedShellNames = []string{"SQL", "TQL", "WORKSHEET", "TAG ANALYZER", "SHELL",
	/*and more for future uses*/ "WORKBOOK", "SCRIPT", "RUN", "CMD", "COMMAND", "CONSOLE",
	/*and more for future uses*/ "MONITOR", "CHART", "DASHBOARD", "LOG", "HOME", "PLAYGROUND"}

var reservedWebShellDef = map[string]*model.ShellDefinition{
	SHELLID_SQL: {Type: "sql", Label: "SQL", Icon: "file-document-outline", Id: SHELLID_SQL},
	SHELLID_TQL: {Type: "tql", Label: "TQL", Icon: "chart-scatter-plot", Id: SHELLID_TQL},
	SHELLID_WRK: {Type: "wrk", Label: "WORKSHEET", Icon: "clipboard-text-play-outline", Id: SHELLID_WRK},
	SHELLID_TAZ: {Type: "taz", Label: "TAG ANALYZER", Icon: "chart-line", Id: SHELLID_TAZ},
	SHELLID_SHELL: {Type: SHELLTYPE_TERM, Label: "SHELL", Icon: "console", Id: SHELLID_SHELL,
		Attributes: &model.ShellAttributes{Cloneable: true},
	},
}

func (s *svr) initShellProvider() {
	candidates := []string{}
	for _, addr := range s.conf.Grpc.Listeners {
		if runtime.GOOS == "windows" && strings.HasPrefix(addr, "unix://") {
			continue
		}
		candidates = append(candidates, addr)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if strings.HasPrefix(candidates[i], "unix://") {
			return true
		}
		if candidates[i] == "127.0.0.1" || candidates[i] == "localhost" {
			return true
		}
		return false
	})
	if len(candidates) == 0 {
		s.log.Warn("no port found for internal communication")
		return
	}

	shellCmd := ""
	if len(os.Args) > 0 {
		if exename, err := os.Executable(); err != nil {
			shellCmd = os.Args[0]
		} else {
			shellCmd = exename
		}
	}
	reservedWebShellDef[SHELLID_SHELL].Command = fmt.Sprintf(`"%s" shell --server %s`, shellCmd, candidates[0])
}

type OldShellDef struct {
	Args []string `json:"args,omitempty"`
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
			def.Type = SHELLTYPE_TERM
			def.Label = def.Id
			old := &OldShellDef{}
			if err := json.Unmarshal(content, old); err == nil && len(old.Args) > 0 {
				def.Command = strings.Join(old.Args, " ")
			}
			if def.Attributes == nil {
				def.Attributes = &model.ShellAttributes{
					Cloneable: true, Removable: true, Editable: true,
				}
			}
		}
		if def.Icon == "" {
			def.Icon = "console-network-outline"
		}
		if def.Label == "" {
			def.Label = "CUSTOM SHELL"
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
func (s *svr) provideShellForSsh(user string, shellId string) *sshd.Shell {
	shellId = strings.ToUpper(shellId)
	var shellDef *model.ShellDefinition
	if shellId == SHELLID_SHELL {
		shellDef = reservedWebShellDef[SHELLID_SHELL]
	}
	if shellDef == nil {
		s.IterateShellDefs(func(def *model.ShellDefinition) bool {
			if def.Id == shellId {
				shellDef = def
				return false
			}
			return true
		})
	}
	if shellDef == nil {
		return nil
	}

	parsed := util.SplitFields(shellDef.Command, true)
	if len(parsed) == 0 {
		return nil
	}

	shell := &sshd.Shell{}

	shell.Cmd = parsed[0]
	if len(parsed) > 1 {
		shell.Args = parsed[1:]
	}

	shell.Envs = map[string]string{}
	envs := os.Environ()
	for _, line := range envs {
		toks := strings.SplitN(line, "=", 2)
		if len(toks) != 2 {
			continue
		}
		shell.Envs[toks[0]] = toks[1]
	}
	if runtime.GOOS == "windows" {
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

func (s *svr) GetAllWebShells() []*model.ShellDefinition {
	var ret []*model.ShellDefinition
	ret = append(ret, reservedWebShellDef[SHELLID_SQL])
	// ret = append(ret, reservedWebShellDef[SHELLID_TQL])
	// ret = append(ret, reservedWebShellDef[SHELLID_WRK])
	ret = append(ret, reservedWebShellDef[SHELLID_TAZ])
	def := reservedWebShellDef[SHELLID_SHELL]
	def.Attributes.Cloneable = false
	ret = append(ret, def)
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
		ret.Type = SHELLTYPE_TERM
		ret.Attributes = &model.ShellAttributes{Removable: true, Editable: true, Cloneable: true}
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

func (s *svr) WebRecents() []httpd.WebReferenceGroup {
	ret := []httpd.WebReferenceGroup{}

	recents := httpd.WebReferenceGroup{Label: "Open recent..."}
	sf := ssfs.Default()
	for _, recent := range sf.GetRecentList() {
		typ := ""
		if idx := strings.LastIndex(recent, "."); idx > 0 && len(recent) > idx+1 {
			typ = recent[idx+1:]
		}
		if typ == "" {
			continue
		}
		recents.Items = append(recents.Items, httpd.ReferenceItem{
			Type:  typ,
			Title: strings.TrimPrefix(recent, "/"),
			Addr:  fmt.Sprintf("serverfile://%s", recent),
		})
	}
	if len(recents.Items) > 0 {
		ret = append(ret, recents)
	}
	return ret
}

func (s *svr) WebReferences() []httpd.WebReferenceGroup {
	ret := []httpd.WebReferenceGroup{}

	references := httpd.WebReferenceGroup{Label: "References"}
	// references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://neo.machbase.com/", Target: "_blank"})
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "https://docs.machbase.com/en/", Target: "_blank"})
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_blank"})
	ret = append(ret, references)

	// tutorials := httpd.WebReferenceGroup{Label: "Tutorials"}
	// tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "Waves in TQL", Addr: "./tutorials/waves_in_tql.wrk"})
	// tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "Fast Fourier Transform in TQL", Addr: "./tutorials/fft_in_tql.wrk"})
	// ret = append(ret, tutorials)

	// samples := httpd.WebReferenceGroup{Label: "Samples"}
	// samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "wrk", Title: "markdown cheatsheet", Addr: "./tutorials/sample_markdown.wrk"})
	// samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "wrk", Title: "mermaid cheatsheet", Addr: "./tutorials/sample_mermaid.wrk"})
	// samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "wrk", Title: "pikchr cheatsheet", Addr: "./tutorials/sample_pikchr.wrk"})
	// samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "tql", Title: "user script in tql (1)", Addr: "./tutorials/user-script1.tql"})
	// samples.Items = append(samples.Items, httpd.ReferenceItem{Type: "tql", Title: "user script in tql (2)", Addr: "./tutorials/user-script2.tql"})
	// ret = append(ret, samples)

	return ret
}

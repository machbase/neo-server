package server

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/machbase/neo-server/mods/service/httpd"
	"github.com/machbase/neo-server/mods/service/sshd"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/ssfs"
)

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
	s.models.ShellProvider().SetDefaultShellCommand(fmt.Sprintf(`"%s" shell --server %s`, shellCmd, candidates[0]))
}

// sshd shell provider
func (s *svr) provideShellForSsh(user string, shellId string) *sshd.Shell {
	shellId = strings.ToUpper(shellId)
	shellDef, _ := s.models.ShellProvider().GetShell(shellId)
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
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://neo.machbase.com/", Target: "_blank"})
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "https://docs.machbase.com/en/", Target: "_blank"})
	references.Items = append(references.Items, httpd.ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_blank"})
	ret = append(ret, references)

	tutorials := httpd.WebReferenceGroup{Label: "Tutorials"}
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "Waves in TQL", Addr: "./tutorials/waves_in_tql.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "Fast Fourier Transform in TQL", Addr: "./tutorials/fft_in_tql.wrk"})

	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "TQL-1 : Glance TQL", Addr: "./tutorials/TQL-Glance.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "TQL-2 : Fast Fourier Transform in TQL", Addr: "./tutorials/TQL-FFT.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "TQL-3 : User Script in TQL", Addr: "./tutorials/TQL-User-Script.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "TQL-4 : User data formats in TQL", Addr: "./tutorials/TQL-User-Data-Format.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "TQL-5 : Query Parameter in TQL", Addr: "./tutorials/TQL-Query-Parameter.wrk"})
	tutorials.Items = append(tutorials.Items, httpd.ReferenceItem{Type: "wrk", Title: "TQL-6 : Time Manipulation in TQL", Addr: "./tutorials/TQL-Time-Manipulation.wrk"})

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

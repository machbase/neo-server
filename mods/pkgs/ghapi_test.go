package pkgs_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/machbase/neo-server/mods/pkgs"
)

func ExampleGithubRepoInfo() {
	nfo, err := pkgs.GithubRepoInfo(http.DefaultClient, "machbase", "neo-pkg-web-example")
	if err != nil {
		panic(err)
	}
	if nfo == nil {
		panic("repo not found")
	}

	sb := &strings.Builder{}
	enc := json.NewEncoder(sb)
	enc.SetIndent("", "  ")
	enc.Encode(nfo)

	fmt.Println(sb.String())

	// Output:
	//{
	//   "organization": "machbase",
	//   "repo": "neo-pkg-web-example",
	//   "name": "neo-pkg-web-example",
	//   "full_name": "machbase/neo-pkg-web-example",
	//   "description": "",
	//   "homepage": "",
	//   "language": "JavaScript",
	//   "license": "",
	//   "default_branch": "main"
	//}
	//
}

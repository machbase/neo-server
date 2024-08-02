package pkgs_test

import (
	"fmt"
	"testing"

	"github.com/machbase/neo-server/mods/pkgs"
)

func TestBuild(t *testing.T) {
	t.Skip("Skip test")
	builder, err := pkgs.NewBuilder(nil, "0.0.1",
		pkgs.WithWorkDir("./tmp/builder"),
		pkgs.WithDistDir("./tmp/dist"),
	)
	if err != nil {
		panic(err)
	}
	err = builder.Build("latest")
	if err != nil {
		panic(err)
	}
	fmt.Println("Build successful")
	// Output:
	// &{Orgnization:machbase Repo:neo-pkg-web-example Name:v0.0.1 TagName:v0.0.1 PublishedAt:2024-07-29 05:17:51 +0000 UTC HtmlUrl:https://github.com/machbase/neo-pkg-web-example/releases/tag/v0.0.1 TarballUrl:https://api.github.com/repos/machbase/neo-pkg-web-example/tarball/v0.0.1 Prerelease:false}
	// Build successful
}

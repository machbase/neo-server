//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	_ "github.com/magefile/mage/mage"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

var Aliases = map[string]any{
	"machbase-neo": Build,
	"neow":         BuildNeow,
	"neoshell":     BuildNeoShell,
	"cleanpackage": CleanPackage,
}

var vLastVersion string
var vLastCommit string
var vIsNightly bool
var vBuildVersion string

func Build() error {
	mg.Deps(GetVersion)
	return build("machbase-neo")
}

func BuildNeow() error {
	mg.Deps(GetVersion)
	return build("neow")
}

func BuildNeoShell() error {
	mg.Deps(GetVersion)
	return build("neoshell")
}

func build(target string) error {
	fmt.Println("Build", target, vBuildVersion, "...")

	mod := "github.com/machbase/neo-server"
	edition := "standard"
	timestamp := time.Now().Format("2006-01-02T15:04:05")
	gitSHA := vLastCommit[0:8]
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	env := map[string]string{"GO111MODULE": "on"}
	if target != "neoshell" {
		env["CGO_ENABLE"] = "1"
	} else {
		env["CGO_ENABLE"] = "0"
	}

	args := []string{"build"}
	if target != "neow" {
		ldflags := strings.Join([]string{
			"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
			"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
			"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
			"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
			"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
		}, " ")
		args = append(args, "-ldflags", ldflags)
	}
	// executable file
	if runtime.GOOS == "windows" {
		args = append(args, "-o", fmt.Sprintf("./tmp/%s.exe", target))
	} else {
		args = append(args, "-o", fmt.Sprintf("./tmp/%s", target))
	}
	// source directory
	args = append(args, fmt.Sprintf("./main/%s", target))

	if err := sh.RunV("go", "mod", "tidy"); err != nil {
		return err
	}

	err := sh.RunWithV(env, "go", args...)
	if err != nil {
		return err
	}
	fmt.Println("Build done.")
	return nil
}

func Test() error {
	if err := sh.RunV("go", "test", "./...", "-cover", "-coverprofile", "./tmp/cover.out"); err != nil {
		return err
	}
	fmt.Println("Test done.")
	return nil
}

func Package() error {
	target := "machbase-neo"
	mg.Deps(CleanPackage, GetVersion)

	bdir := fmt.Sprintf("%s-%s-%s-%s", target, vBuildVersion, runtime.GOOS, runtime.GOARCH)
	if runtime.GOARCH == "arm" {
		bdir = fmt.Sprintf("%s-%s-%s-arm32", target, vBuildVersion, runtime.GOOS)
	}
	os.RemoveAll(filepath.Join("./packages", bdir))
	os.Mkdir(filepath.Join("./packages", bdir), 0755)

	if err := build("machbase-neo"); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := build("neow"); err != nil {
			return err
		}
		os.Rename("./tmp/machbase-neo.exe", filepath.Join("./packages", bdir, "machbase-neo.exe"))
		os.Rename("./tmp/neow.exe", filepath.Join("./packages", bdir, "neow.exe"))
	} else {
		err := os.Rename("./tmp/machbase-neo", filepath.Join("./packages", bdir, "machbase-neo"))
		if err != nil {
			return err
		}
	}

	err := sh.RunV("zip", "-r", "-q",
		fmt.Sprintf("./packages/%s.zip", bdir),
		filepath.Join("./packages", bdir))
	if err != nil {
		return err
	}

	os.RemoveAll(filepath.Join("./packages", bdir))
	return nil
}

func CleanPackage() error {
	entries, err := os.ReadDir("./packages")
	if err != nil {
		return err
	}

	for _, ent := range entries {
		if err = os.Remove(filepath.Join("./packages", ent.Name())); err != nil {
			return err
		}
	}
	return nil
}

func GetVersion() error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return err
	}
	headRef, err := repo.Head()
	if err != nil {
		return err
	}
	// if headRef.Name().IsBranch() {
	// 	fmt.Println("branch  ", headRef.Name().Short())
	// } else if headRef.Name().IsTag() {
	// 	fmt.Println("tag     ", headRef.Name().Short())
	// }

	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return err
	}

	var lastTag *object.Tag
	tagiter, err := repo.TagObjects()
	err = tagiter.ForEach(func(tag *object.Tag) error {
		tagCommit, err := tag.Commit()
		if err != nil {
			return err
		}
		if lastTag == nil {
			lastTag = tag
		} else {
			lastCommit, _ := lastTag.Commit()
			if tagCommit.Committer.When.Sub(lastCommit.Committer.When) > 0 {
				lastTag = tag
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	lastTagCommit, err := lastTag.Commit()
	if err != nil {
		return err
	}
	vLastVersion = lastTag.Name
	vLastCommit = commit.Hash.String()
	vIsNightly = lastTagCommit.Hash.String() != vLastCommit
	lastVer, err := semver.NewVersion(vLastVersion)
	if err != nil {
		return err
	}
	if vIsNightly {
		vBuildVersion = fmt.Sprintf("v%d.%d.%d-snapshot", lastVer.Major(), lastVer.Minor(), lastVer.Patch()+1)
	} else {
		vBuildVersion = fmt.Sprintf("v%d.%d.%d", lastVer.Major(), lastVer.Minor(), lastVer.Patch())
	}

	return nil
}

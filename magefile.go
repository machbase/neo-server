//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	_ "github.com/magefile/mage/mage"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

func GetVersion() error {
	fmt.Println("GOOS    ", runtime.GOOS)
	fmt.Println("GOARCH  ", runtime.GOARCH)
	repo, err := git.PlainOpen(".")
	if err != nil {
		return err
	}
	headRef, err := repo.Head()
	if err != nil {
		return err
	}
	if headRef.Name().IsBranch() {
		fmt.Println("branch  ", headRef.Name().Short())
	} else if headRef.Name().IsTag() {
		fmt.Println("tag     ", headRef.Name().Short())
	}

	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return err
	}
	fmt.Println("commit  ", commit.Hash)
	fmt.Println("author  ", commit.Committer.Name)
	fmt.Println("when    ", commit.Committer.When.Format("Mon Jan 02 15:04:05 2006 -0700"))

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
	lastTagCommit, _ := lastTag.Commit()
	fmt.Println("lastTag ", lastTag.Name, lastTag.Hash, lastTagCommit.Committer.When.Format("Mon Jan 02 15:04:05 2006 -0700"))
	return nil
}

func Test() error {
	if err := sh.RunV("go", "test", "./...", "-cover", "-coverprofile", "./tmp/cover.out"); err != nil {
		return err
	}
	return nil
}

func Build() error {
	mg.Deps(GetVersion)
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

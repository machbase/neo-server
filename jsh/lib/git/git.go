package git

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

//go:embed git.js
var gitJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"git.js": gitJS,
	}
}

type hostPathResolver interface {
	ResolveHostPath(name string) (string, error)
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	installModule(rt, module, nil)
}

func ModuleWithFS(resolver hostPathResolver) func(context.Context, *goja.Runtime, *goja.Object) {
	return func(_ context.Context, rt *goja.Runtime, module *goja.Object) {
		installModule(rt, module, resolver)
	}
}

func installModule(rt *goja.Runtime, module *goja.Object, resolver hostPathResolver) {
	m := module.Get("exports").(*goja.Object)
	m.Set("listRemoteRefs", func(url string) ([]map[string]any, error) {
		return listRemoteRefs(strings.TrimSpace(url))
	})
	m.Set("cloneRepository", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(rt.NewTypeError("url and dir are required"))
		}
		url := strings.TrimSpace(call.Arguments[0].String())
		dir := strings.TrimSpace(call.Arguments[1].String())
		if resolver != nil {
			resolved, err := resolver.ResolveHostPath(dir)
			if err != nil {
				panic(rt.NewGoError(err))
			}
			dir = resolved
		}
		if url == "" || dir == "" {
			panic(rt.NewTypeError("url and dir are required"))
		}
		opts := readCloneOptions(rt, goja.Undefined())
		if len(call.Arguments) > 2 {
			opts = readCloneOptions(rt, call.Arguments[2])
		}
		result, err := cloneRepository(url, dir, opts)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(result)
	})
}

type cloneOptions struct {
	Ref          string
	RefType      string
	Depth        int
	SingleBranch bool
	RemoveGitDir bool
}

func readCloneOptions(rt *goja.Runtime, value goja.Value) cloneOptions {
	opts := cloneOptions{Depth: 1, SingleBranch: true, RemoveGitDir: false}
	if goja.IsUndefined(value) || goja.IsNull(value) {
		return opts
	}
	obj := value.ToObject(rt)
	if obj == nil {
		return opts
	}
	if prop := obj.Get("ref"); !goja.IsUndefined(prop) && !goja.IsNull(prop) {
		opts.Ref = strings.TrimSpace(prop.String())
	}
	if prop := obj.Get("refType"); !goja.IsUndefined(prop) && !goja.IsNull(prop) {
		opts.RefType = strings.TrimSpace(prop.String())
	}
	if prop := obj.Get("depth"); !goja.IsUndefined(prop) && !goja.IsNull(prop) {
		if n := int(prop.ToInteger()); n > 0 {
			opts.Depth = n
		}
	}
	if prop := obj.Get("singleBranch"); !goja.IsUndefined(prop) && !goja.IsNull(prop) {
		opts.SingleBranch = prop.ToBoolean()
	}
	if prop := obj.Get("removeGitDir"); !goja.IsUndefined(prop) && !goja.IsNull(prop) {
		opts.RemoveGitDir = prop.ToBoolean()
	}
	return opts
}

func listRemoteRefs(url string) ([]map[string]any, error) {
	if url == "" {
		return nil, fmt.Errorf("git remote URL is required")
	}
	remote := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	refs, err := remote.List(&gogit.ListOptions{})
	if err != nil {
		return nil, err
	}
	ret := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		item := map[string]any{
			"name":      ref.Name().String(),
			"shortName": ref.Name().Short(),
			"hash":      ref.Hash().String(),
			"isBranch":  ref.Name().IsBranch(),
			"isTag":     ref.Name().IsTag(),
			"isRemote":  ref.Name().IsRemote(),
			"isHEAD":    ref.Name() == plumbing.HEAD,
		}
		if ref.Type() == plumbing.SymbolicReference {
			item["target"] = ref.Target().String()
		}
		ret = append(ret, item)
	}
	return ret, nil
}

func cloneRepository(url string, dir string, opts cloneOptions) (map[string]any, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, err
	}

	cloneOpts := &gogit.CloneOptions{
		URL:          url,
		Depth:        opts.Depth,
		SingleBranch: opts.SingleBranch,
	}
	if opts.Ref != "" {
		switch opts.RefType {
		case "branch":
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Ref)
			cloneOpts.SingleBranch = true
		case "tag":
			cloneOpts.ReferenceName = plumbing.NewTagReferenceName(opts.Ref)
			cloneOpts.SingleBranch = false
		default:
			cloneOpts.ReferenceName = plumbing.ReferenceName(opts.Ref)
		}
	}

	repo, err := gogit.PlainClone(dir, false, cloneOpts)
	if err != nil && opts.RefType == "tag" && opts.Depth > 0 {
		_ = os.RemoveAll(dir)
		if mkErr := os.MkdirAll(filepath.Dir(dir), 0o755); mkErr != nil {
			return nil, mkErr
		}
		cloneOpts.Depth = 0
		repo, err = gogit.PlainClone(dir, false, cloneOpts)
	}
	if err != nil {
		return nil, err
	}

	headRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	if opts.RemoveGitDir {
		if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"headRef":  headRef.Name().String(),
		"headHash": headRef.Hash().String(),
	}, nil
}

package pkgs

import (
	"os"

	"gopkg.in/yaml.v3"
)

type PackageMeta struct {
	InjectRecipe    InjectRecipe     `yaml:"inject" json:"inject"`
	Distributable   Distributable    `yaml:"distributable" json:"distributable"`
	Description     string           `yaml:"description" json:"description"`
	BuildRecipe     BuildRecipe      `yaml:"build" json:"build"`
	Provides        []string         `yaml:"provides" json:"provides"`
	TestRecipe      *TestRecipe      `yaml:"test,omitempty" json:"test,omitempty"`
	InstallRecipe   *InstallRecipe   `yaml:"install,omitempty" json:"install,omitempty"`
	UninstallRecipe *UninstallRecipe `yaml:"uninstall,omitempty" json:"uninstall,omitempty"`

	rosterName RosterName `json:"-"`
}

type InjectRecipe struct {
	Type string `yaml:"type"`
}

type Distributable struct {
	Github          string `yaml:"github"`
	Url             string `yaml:"url"`
	StripComponents int    `yaml:"strip_components"`
}

type BuildRecipe struct {
	Script []string `yaml:"script"`
	Env    []string `yaml:"env"`
}

type TestRecipe struct {
	Script string `yaml:"script"`
}

type InstallRecipe struct {
	Script string `yaml:"script"`
}

type UninstallRecipe struct {
	Script string `yaml:"script"`
}

func parsePackageMetaFile(path string) (*PackageMeta, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &PackageMeta{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

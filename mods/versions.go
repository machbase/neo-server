package mods

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	mach "github.com/machbase/neo-engine"
)

var (
	versionString   = ""
	versionGitSHA   = ""
	buildTimestamp  = ""
	goVersionString = ""
	editionString   = ""
)

type Version struct {
	Major  int    `json:"major"`
	Minor  int    `json:"minor"`
	Patch  int    `json:"patch"`
	GitSHA string `json:"git"`

	Edition string `json:"edition"`
}

var _version *Version

func GetVersion() *Version {
	if _version == nil {
		v, err := semver.NewVersion(versionString)
		if err != nil {
			_version = &Version{}
		} else {
			_version = &Version{
				Major:   int(v.Major()),
				Minor:   int(v.Minor()),
				Patch:   int(v.Patch()),
				GitSHA:  versionGitSHA,
				Edition: editionString,
			}
		}
	}
	return _version
}

func VersionString() string {
	return fmt.Sprintf("%s (%v, %v)", versionString, versionGitSHA, buildTimestamp)
}

func BuildCompiler() string {
	return goVersionString
}

func BuildTimestamp() string {
	return buildTimestamp
}

func EngineInfoString() string {
	return mach.LibMachLinkInfo
}

func Edition() string {
	return editionString
}

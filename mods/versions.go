package mods

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/machbase/neo-server/api/machsvr"
	"github.com/machbase/neo-server/mods/util"
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

func DisplayVersion() string {
	return versionString
}

func VersionString() string {
	return fmt.Sprintf("%s (%v %v)", versionString, versionGitSHA, buildTimestamp)
}

func BuildCompiler() string {
	return goVersionString
}

func BuildTimestamp() string {
	return buildTimestamp
}

func Edition() string {
	return editionString
}

//go:embed banner_c.txt
var bannerAnsic string

//go:embed banner_m.txt
var bannerPlain string

func GenBanner() string {
	supportColor := true
	windowsVersion := ""
	if runtime.GOOS == "windows" {
		major, minor, build := util.GetWindowsVersion()
		windowsVersion = fmt.Sprintf("Windows %d.%d %d", major, minor, build)
		if major <= 10 && build < 14931 {
			supportColor = false
		}
	}

	logo := bannerAnsic
	if !supportColor {
		logo = bannerPlain
	}

	logo = strings.ReplaceAll(logo, "\r\n", "\n")
	lines := strings.Split(logo, "\n")
	lines[6] = lines[6] + fmt.Sprintf("  %s", VersionString())
	lines[7] = lines[7] + fmt.Sprintf("  engine v%s (%s)", machsvr.LinkVersion(), machsvr.LinkGitHash())
	lines[8] = lines[8] + fmt.Sprintf("  %s %s", machsvr.LinkInfo(), windowsVersion)
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

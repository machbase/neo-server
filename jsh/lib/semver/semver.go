package semver

import (
	_ "embed"
	"strings"

	masterminds "github.com/Masterminds/semver/v3"
	"github.com/dop251/goja"
)

//go:embed semver.js
var semverJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"semver.js": semverJS,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	o.Set("satisfies", func(version string, constraint string) bool {
		ok, err := satisfies(version, constraint)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return ok
	})
	o.Set("maxSatisfying", func(versions []string, constraint string) string {
		matched, err := maxSatisfying(versions, constraint)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return matched
	})
	o.Set("compare", func(left string, right string) int {
		cmp, err := compare(left, right)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return cmp
	})
}

func satisfies(version string, constraint string) (bool, error) {
	parsedVersion, err := masterminds.NewVersion(strings.TrimSpace(version))
	if err != nil {
		return false, err
	}
	parsedConstraint, err := masterminds.NewConstraint(normalizeConstraint(constraint))
	if err != nil {
		return false, err
	}
	return parsedConstraint.Check(parsedVersion), nil
}

func maxSatisfying(versions []string, constraint string) (string, error) {
	parsedConstraint, err := masterminds.NewConstraint(normalizeConstraint(constraint))
	if err != nil {
		return "", err
	}
	var matched *masterminds.Version
	matchedRaw := ""
	for _, candidate := range versions {
		parsedVersion, err := masterminds.NewVersion(strings.TrimSpace(candidate))
		if err != nil {
			continue
		}
		if !parsedConstraint.Check(parsedVersion) {
			continue
		}
		if matched == nil || parsedVersion.GreaterThan(matched) {
			matched = parsedVersion
			matchedRaw = candidate
		}
	}
	return matchedRaw, nil
}

func compare(left string, right string) (int, error) {
	parsedLeft, err := masterminds.NewVersion(strings.TrimSpace(left))
	if err != nil {
		return 0, err
	}
	parsedRight, err := masterminds.NewVersion(strings.TrimSpace(right))
	if err != nil {
		return 0, err
	}
	if parsedLeft.LessThan(parsedRight) {
		return -1, nil
	}
	if parsedLeft.GreaterThan(parsedRight) {
		return 1, nil
	}
	return 0, nil
}

func normalizeConstraint(constraint string) string {
	trimmed := strings.TrimSpace(constraint)
	if trimmed == "" || trimmed == "latest" {
		return "*"
	}
	return trimmed
}
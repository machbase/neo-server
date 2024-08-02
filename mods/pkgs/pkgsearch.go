package pkgs

import (
	"slices"
	"strings"
)

type PackageSearch struct {
	Name  string
	Score float32
}

type PackageSearchResult struct {
	ExactMatch *PackageCache   `json:"exact"`
	Possibles  []*PackageCache `json:"possibles"`
}

// Search package info by name, if it finds the package, return the package info.
// if not found it will return similar package names.
// if there is no similar package names, it will return empty string slice.
// if possibles is 0, it will only return exact match.
func (r *Roster) SearchPackage(name string, possibles int) (*PackageSearchResult, error) {
	nfo, err := r.LoadPackageMeta(name)
	if err != nil {
		return nil, err
	}
	ret := &PackageSearchResult{}
	if nfo != nil {
		cache, err := r.LoadPackageCache(name, nfo, false)
		if err != nil {
			return nil, err
		}
		ret.ExactMatch = cache
	}
	if possibles == 0 {
		return ret, nil
	}
	// search similar package names
	candidates := []*PackageSearch{}
	r.cacheManagers[ROSTER_CENTRAL].Walk(func(nm string) bool {
		score := CompareTwoStrings(strings.ToLower(nm), name)
		if score > 0.1 {
			candidates = append(candidates, &PackageSearch{Name: nm, Score: score})
		}
		return true
	})

	slices.SortFunc(candidates, func(a, b *PackageSearch) int {
		if a.Score > b.Score {
			return -1
		} else if a.Score < b.Score {
			return 1
		}
		return 0
	})
	if len(candidates) > possibles {
		candidates = candidates[:possibles]
	}
	for _, c := range candidates {
		cache, err := r.cacheManagers[ROSTER_CENTRAL].ReadCache(c.Name)
		if err != nil {
			continue
		}
		ret.Possibles = append(ret.Possibles, cache)
	}
	return ret, nil
}

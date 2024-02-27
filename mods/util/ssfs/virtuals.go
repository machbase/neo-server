package ssfs

import "fmt"

const urlGitSample = "https://github.com/machbase/neo-tutorials.git"
const enableGitSample = false

func appendVirtual(path string, entry *Entry) *Entry {
	if path == "/" && enableGitSample {
		// Add Git Sample repo to root directory
		hasSamples := false
		for _, child := range entry.Children {
			if !child.IsDir || !child.GitClone {
				continue
			}
			switch child.GitUrl {
			case urlGitSample:
				hasSamples = true
			}
		}
		if !hasSamples {
			nameSamples := "Tutorials"
			count := 0
		reRun:
			for _, child := range entry.Children {
				if child.Name == nameSamples {
					count++
					nameSamples = fmt.Sprintf("Tutorials-%d", count)
					goto reRun
				}
			}
			entry.Children = append(entry.Children, &SubEntry{
				IsDir:              true,
				Name:               nameSamples,
				Type:               "dir",
				Size:               0,
				LastModifiedMillis: 0,
				GitUrl:             urlGitSample,
				GitClone:           true,
				Virtual:            true,
			})
		}
	}
	return entry
}

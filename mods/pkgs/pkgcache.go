package pkgs

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type PackageCache struct {
	Name             string      `yaml:"name" json:"name"`
	Github           *GhRepoInfo `yaml:"github" json:"github"`
	LatestRelease    string      `yaml:"latest_release" json:"latest_release"`
	LatestReleaseTag string      `yaml:"latest_release_tag" json:"latest_release_tag"`
	PublishedAt      time.Time   `yaml:"published_at" json:"published_at"`
	Url              string      `yaml:"url" json:"url"`
	StripComponents  int         `yaml:"strip_components" json:"strip_components"`
	CachedAt         time.Time   `yaml:"cached_at" json:"cached_at"`
	InstalledVersion string      `yaml:"installed_version" json:"installed_version"`
	InstalledPath    string      `yaml:"installed_path" json:"installed_path"`
}

type CacheManager struct {
	cacheDir string
}

func NewCacheManager(cacheDir string) *CacheManager {
	return &CacheManager{cacheDir: cacheDir}
}

func (cm *CacheManager) ReadCache(name string) (*PackageCache, error) {
	path := filepath.Join(cm.cacheDir, name, "cache.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &PackageCache{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (cm *CacheManager) WriteCache(cache *PackageCache) error {
	cacheDir := filepath.Join(cm.cacheDir, cache.Name)
	if _, err := os.Stat(cacheDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(cacheDir, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	cacheFile := filepath.Join(cacheDir, "cache.yml")
	content, err := yaml.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, content, 0644)
}

func (cm *CacheManager) Walk(cb func(name string) bool) error {
	entries, err := os.ReadDir(cm.cacheDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if !cb(entry.Name()) {
				return nil
			}
		}
	}
	return nil
}

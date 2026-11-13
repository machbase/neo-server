package main

import (
	"archive/zip"
	_ "embed"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	_ "github.com/magefile/mage/mage"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = BuildNeoServer

var Aliases = map[string]any{
	"machbase-neo":       BuildNeoServerDebug,
	"machbase-neo-debug": BuildNeoServerDebug,
	"machbase-neo-strip": BuildNeoServer,
	"neo-shell":          BuildNeoShell,
	"cleanpackage":       CleanPackage,
	"buildversion":       BuildVersion,
	"install-neo-web":    InstallNeoWeb,
	"install-neo-engine": InstallNeoEngine,
}

var vLastVersion string
var vLastCommit string
var vIsNightly bool
var vBuildVersion string

const unknownGitSHA = "unknown"

func BuildVersion() {
	mg.Deps(GetVersion)
	fmt.Println(vBuildVersion)
}

func BuildNeoServer() error {
	return Build("machbase-neo", true)
}

func BuildNeoServerDebug() error {
	return Build("machbase-neo", false)
}

func BuildNeoShell() error {
	return Build("neo-shell", true)
}

func Build(target string, strip bool) error {
	mg.Deps(CheckTmp, GetVersion)

	if target == "machbase-neo" {
		targetOS, targetArch := goTarget()
		if err := ensureBuildArtifacts(targetOS, targetArch); err != nil {
			return err
		}
	}

	fmt.Println("Build", target, vBuildVersion, "...")

	mod := "github.com/machbase/neo-server/v8"
	edition := "standard"
	timestamp := time.Now().Format("2006-01-02T15:04:05")
	gitSHA := shortGitSHA()
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	env := map[string]string{"GO111MODULE": "on"}
	args := []string{"build"}
	if vLastCommit == unknownGitSHA {
		args = append(args, "-buildvcs=false")
	}
	ldflags := strings.Join([]string{
		"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
		"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
		"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
		"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
		"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
		"-X", fmt.Sprintf("%s/spi/mach/native.Version=%s", mod, strings.TrimSpace(neo_engine_version)),
	}, " ")
	if strip {
		// this may reduce binary size about (110M -> 86M)
		ldflags += " -s -w"
	}
	args = append(args, "-ldflags", ldflags)

	// executable file
	if runtime.GOOS == "windows" {
		args = append(args, "-tags=timetzdata")
		args = append(args, "-o", fmt.Sprintf("./tmp/%s.exe", target))
	} else if runtime.GOOS == "linux" && !strip {
		args = append(args, "-tags=debug")
		args = append(args, "-o", fmt.Sprintf("./tmp/%s", target))
	} else {
		args = append(args, "-o", fmt.Sprintf("./tmp/%s", target))
	}
	// source directory
	args = append(args, fmt.Sprintf("./cmd/%s", target))

	if err := sh.RunV("go", "mod", "download"); err != nil {
		return err
	}

	err := sh.RunWithV(env, "go", args...)
	if err != nil {
		return err
	}
	fmt.Println("Build done.")
	return nil
}

func BuildX(target string, targetOS string, targetArch string) error {
	mg.Deps(GetVersion, CheckTmp)
	if target == "machbase-neo" {
		if err := ensureBuildArtifacts(targetOS, targetArch); err != nil {
			return err
		}
	}
	fmt.Println("Build", target, vBuildVersion, "...")

	zigVer, err := sh.Output("zig", "version")
	if err != nil {
		fmt.Println("error: Zig is not installed. Please download and follow installation")
		fmt.Println("instructions at https://ziglang.org/ to continue.")
		return err
	}
	fmt.Println("zig", zigVer)

	mod := "github.com/machbase/neo-server/v8"
	edition := "standard"
	timestamp := time.Now().Format("2006-01-02T15:04:05")
	gitSHA := shortGitSHA()
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	env := map[string]string{"GO111MODULE": "on"}
	if target == "machbase-neo" {
		env["CGO_ENABLED"] = "1"
	} else {
		env["CGO_ENABLED"] = "0"
	}
	env["GOOS"] = targetOS
	env["GOARCH"] = targetArch
	switch targetOS {
	case "linux":
		switch targetArch {
		case "arm64":
			env["CC"] = "zig cc -target aarch64-linux-gnu"
			env["CXX"] = "zig c++ -target aarch64-linux-gnu"
		case "amd64":
			env["CC"] = "zig cc -target x86_64-linux-gnu"
			env["CXX"] = "zig c++ -target x86_64-linux-gnu"
		case "arm":
			env["CC"] = "zig cc -target arm-linux-gnueabihf"
			env["CXX"] = "zig c++ -target arm-linux-gnueabihf"
		case "386":
			env["CC"] = "zig cc -target x86-linux-gnu"
			env["CXX"] = "zig c++ -target x86-linux-gnu"
		default:
			return fmt.Errorf("error: unsupported linux/%s", targetArch)
		}
	case "darwin":
		sysroot, err := sh.Output("xcrun", "--sdk", "macosx", "--show-sdk-path")
		if err != nil {
			return err
		}
		sysflags := fmt.Sprintf("-v --sysroot=%s -I/usr/include, -F/System/Library/Frameworks -L/usr/lib", sysroot)
		switch targetArch {
		case "arm64":
			env["CC"] = "zig cc -target aarch64-macos.13-none " + sysflags
			env["CXX"] = "zig c++ -target aarch64-macos.13-none " + sysflags
		case "amd64":
			env["CC"] = "zig cc -target x86_64-macos.13-none " + sysflags
			env["CXX"] = "zig c++ -target x86_64-macos.13-none " + sysflags
		default:
			return fmt.Errorf("error: unsupported darwin/%s", targetArch)
		}
	case "windows":
		env["CC"] = "zig cc -target x86_64-windows-none"
		env["CXX"] = "zig c++ -target x86_64-windows-none"
	default:
		return fmt.Errorf("error: unsupported os %s", targetOS)
	}

	args := []string{"build"}
	if vLastCommit == unknownGitSHA {
		args = append(args, "-buildvcs=false")
	}
	ldflags := strings.Join([]string{
		"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
		"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
		"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
		"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
		"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
		"-X", fmt.Sprintf("%s/spi/mach/native.Version=%s", mod, strings.TrimSpace(neo_engine_version)),
	}, " ")
	args = append(args, "-ldflags", ldflags)

	// executable file
	if targetOS == "windows" {
		args = append(args, "-tags=timetzdata")
		args = append(args, "-o", fmt.Sprintf("./tmp/%s.exe", target))
	} else {
		args = append(args, "-o", fmt.Sprintf("./tmp/%s", target))
	}
	// source directory
	args = append(args, fmt.Sprintf("./cmd/%s", target))

	if err := sh.RunV("go", "mod", "download"); err != nil {
		return err
	}

	err = sh.RunWithV(env, "go", args...)
	if err != nil {
		return err
	}
	fmt.Println("Build done.")
	return nil
}

func Test() error {
	return runTest()
}

func TestN(p int) error {
	return runTest("-p", strconv.Itoa(p))
}

func TestAll() error {
	return runTest("-count=1")
}

func runTest(extraArgs ...string) error {
	mg.Deps(CheckTmp)

	targetOS, targetArch := goTarget()
	if err := installArtifact(engineArtifact(neo_engine_version, targetOS, targetArch), false); err != nil {
		return err
	}

	env := map[string]string{
		"GO111MODULE": "on",
		"CGO_ENABLED": "1",
	}

	if runtime.GOOS == "linux" {
		env["CGO_LDFLAGS"] = "-pthread"
	}
	if err := sh.RunWithV(env, "go", "mod", "download"); err != nil {
		return err
	}

	testArgs := []string{
		"test",
	}
	testArgs = append(testArgs, extraArgs...)
	testArgs = append(testArgs,
		"-cover", "-coverprofile", "./tmp/cover.out", "./...",
	)

	if err := sh.RunWithV(env, "go", testArgs...); err != nil {
		return err
	}
	if output, err := sh.Output("go", "tool", "cover", "-func=./tmp/cover.out"); err != nil {
		return err
	} else {
		lines := strings.Split(output, "\n")
		fmt.Println(lines[len(lines)-1])
	}
	fmt.Println("Test done.")
	return nil
}

func CheckTmp() error {
	_, err := os.Stat("tmp")
	if err != nil && err != os.ErrNotExist {
		err = os.Mkdir("tmp", 0755)
	} else if err != nil && err == os.ErrExist {
		return nil
	}
	return err
}

func Generate() error {
	return sh.RunV("go", "generate", "./...")
}

func Protoc() error {
	args := []string{}
	if len(args) == 0 {
		args = []string{
			"bridge",
		}
	}

	for _, mod := range args {
		fmt.Printf("protoc regen api/proto/%s.proto...\n", mod)
		err := sh.RunV("protoc", "-I", "api/proto", mod+".proto",
			"--experimental_allow_proto3_optional",
			fmt.Sprintf("--go_out=./api/%s", mod), "--go_opt=paths=source_relative",
			fmt.Sprintf("--go-grpc_out=./api/%s", mod), "--go-grpc_opt=paths=source_relative",
		)
		if err != nil {
			return fmt.Errorf("error: protoc failed for %s: %w", mod, err)
		}
	}
	return nil
}

func Package() error {
	return PackageX(runtime.GOOS, runtime.GOARCH)
}

func PackageX(targetOS string, targetArch string) error {
	mg.Deps(CleanPackage, GetVersion, CheckTmp)
	bdir := fmt.Sprintf("machbase-neo-%s-%s-%s", vBuildVersion, targetOS, targetArch)
	if targetArch == "arm" {
		bdir = fmt.Sprintf("machbase-neo-%s-%s-arm32", vBuildVersion, targetOS)
	}
	_, err := os.Stat("packages")
	if err != os.ErrNotExist {
		os.RemoveAll(filepath.Join("packages", bdir))
	}
	os.MkdirAll(filepath.Join("packages", bdir), 0755)

	switch targetOS {
	case "windows":
		if err := os.Rename(filepath.Join("tmp", "machbase-neo.exe"), filepath.Join("packages", bdir, "machbase-neo.exe")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join("tmp", "neow.exe"), filepath.Join("packages", bdir, "neow.exe")); err != nil {
			return err
		}
	case "darwin":
		if err := os.Rename(filepath.Join("tmp", "machbase-neo"), filepath.Join("packages", bdir, "machbase-neo")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join("tmp", "neow.app"), filepath.Join("packages", bdir, "neow.app")); err != nil {
			return err
		}
	default:
		if err := copyFile("./tmp/machbase-neo", filepath.Join("./packages", bdir, "machbase-neo"), 0755); err != nil {
			return err
		}
	}

	err = archivePackage(fmt.Sprintf("./packages/%s.zip", bdir), filepath.Join("./packages", bdir))
	if err != nil {
		return err
	}

	os.RemoveAll(filepath.Join("packages", bdir))
	return nil
}

func archivePackage(dst string, src ...string) error {
	archive, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer archive.Close()
	zipWriter := zip.NewWriter(archive)

	for _, file := range src {
		archiveAddEntry(zipWriter, file, fmt.Sprintf("packages%s", string(os.PathSeparator)))
	}
	return zipWriter.Close()
}

func archiveAddEntry(zipWriter *zip.Writer, entry string, prefix string) error {
	stat, err := os.Stat(entry)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		entries, err := os.ReadDir(entry)
		if err != nil {
			return err
		}
		entryName := strings.TrimPrefix(entry, prefix)
		entryName = strings.ReplaceAll(strings.TrimPrefix(entryName, string(filepath.Separator)), "\\", "/")
		entryName = entryName + "/"
		_, err = zipWriter.Create(entryName)
		if err != nil {
			return err
		}
		fmt.Println("Archive D", entryName)
		for _, ent := range entries {
			archiveAddEntry(zipWriter, filepath.Join(entry, ent.Name()), prefix)
		}
	} else {
		fd, err := os.Open(entry)
		if err != nil {
			return err
		}
		defer fd.Close()

		entryName := strings.TrimPrefix(entry, prefix)
		entryName = strings.ReplaceAll(strings.TrimPrefix(entryName, string(filepath.Separator)), "\\", "/")
		fmt.Println("Archive F", entryName)
		finfo, _ := fd.Stat()
		hdr := &zip.FileHeader{
			Name:               entryName,
			UncompressedSize64: uint64(finfo.Size()),
			Method:             zip.Deflate,
			Modified:           finfo.ModTime(),
		}
		hdr.SetMode(finfo.Mode())

		w, err := zipWriter.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, fd); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, fileMode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func CleanPackage() error {
	entries, err := os.ReadDir("./packages")
	if err != nil {
		if err != os.ErrNotExist {
			return nil
		}
	}

	for _, ent := range entries {
		if err = os.RemoveAll(filepath.Join("./packages", ent.Name())); err != nil {
			return err
		}
	}
	return nil
}

func GetVersion() error {
	repo, err := git.PlainOpen(".")
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			_, statErr := os.Lstat(".git")
			if os.IsNotExist(statErr) {
				vLastVersion = ""
				vLastCommit = unknownGitSHA
				vIsNightly = true
				vBuildVersion = "v0.0.0-snapshot.nogit"
				fmt.Fprintln(os.Stderr, "Git repository not found; using", vBuildVersion)
				return nil
			}
			if statErr != nil {
				return statErr
			}
		}
		return err
	}
	headRef, err := repo.Head()
	if err != nil {
		return err
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return err
	}

	tagName, tagCommit, err := nearestVersionTag(repo, headCommit.Hash)
	if err != nil {
		return err
	}
	vLastVersion = tagName
	vLastCommit = headCommit.Hash.String()
	vIsNightly = tagCommit != headCommit.Hash
	lastTagSemVer, err := semver.NewVersion(vLastVersion)
	if err != nil {
		return err
	}

	if lastTagSemVer.Prerelease() == "" {
		if vIsNightly {
			vBuildVersion = fmt.Sprintf("v%d.%d.%d-snapshot", lastTagSemVer.Major(), lastTagSemVer.Minor(), lastTagSemVer.Patch()+1)
		} else {
			vBuildVersion = fmt.Sprintf("v%d.%d.%d", lastTagSemVer.Major(), lastTagSemVer.Minor(), lastTagSemVer.Patch())
		}
	} else {
		suffix := lastTagSemVer.Prerelease()
		if vIsNightly && strings.HasPrefix(suffix, "rc") {
			n, _ := strconv.Atoi(suffix[2:])
			suffix = fmt.Sprintf("rc%d-snapshot", n+1)
		}
		vBuildVersion = fmt.Sprintf("v%d.%d.%d-%s", lastTagSemVer.Major(), lastTagSemVer.Minor(), lastTagSemVer.Patch(), suffix)
	}

	return nil
}

func shortGitSHA() string {
	if len(vLastCommit) > 8 {
		return vLastCommit[:8]
	}
	return vLastCommit
}

func nearestVersionTag(repo *git.Repository, head plumbing.Hash) (string, plumbing.Hash, error) {
	type versionTag struct {
		name    string
		version *semver.Version
	}
	tags := make(map[plumbing.Hash]versionTag)
	iter, err := repo.Tags()
	if err != nil {
		return "", plumbing.ZeroHash, err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if !strings.HasPrefix(name, "v") {
			return nil
		}
		version, err := semver.NewVersion(name)
		if err != nil {
			return nil
		}
		obj, err := repo.Object(plumbing.AnyObject, ref.Hash())
		if err != nil {
			return fmt.Errorf("resolve tag %s: %w", name, err)
		}
		for {
			tag, ok := obj.(*object.Tag)
			if !ok {
				break
			}
			obj, err = tag.Object()
			if err != nil {
				return fmt.Errorf("resolve tag %s: %w", name, err)
			}
		}
		commit, ok := obj.(*object.Commit)
		if !ok {
			return nil
		}
		old, exists := tags[commit.Hash]
		if !exists || version.GreaterThan(old.version) || (version.Equal(old.version) && name > old.name) {
			tags[commit.Hash] = versionTag{name: name, version: version}
		}
		return nil
	})
	if err != nil {
		return "", plumbing.ZeroHash, err
	}

	visited := map[plumbing.Hash]bool{head: true}
	level := []plumbing.Hash{head}
	for len(level) > 0 {
		var best versionTag
		var bestCommit plumbing.Hash
		for _, hash := range level {
			tag, ok := tags[hash]
			if ok && (best.version == nil || tag.version.GreaterThan(best.version) ||
				(tag.version.Equal(best.version) && tag.name > best.name)) {
				best, bestCommit = tag, hash
			}
		}
		if best.version != nil {
			return best.name, bestCommit, nil
		}
		var next []plumbing.Hash
		for _, hash := range level {
			commit, err := repo.CommitObject(hash)
			if err != nil {
				return "", plumbing.ZeroHash, fmt.Errorf("read commit %s: %w", hash, err)
			}
			for _, parent := range commit.ParentHashes {
				if !visited[parent] {
					visited[parent] = true
					next = append(next, parent)
				}
			}
		}
		level = next
	}
	return "", plumbing.ZeroHash, fmt.Errorf("no version tag reachable from %s", head)
}

//go:embed neo-launcher-version.txt
var neo_launcher_version string

func InstallNeoLauncher() error {
	return InstallNeoLauncherX(neo_launcher_version)
}

func InstallNeoLauncherX(version string) error {
	mg.Deps(CheckTmp)

	var url string
	if runtime.GOOS == "darwin" {
		// universal binary
		url = fmt.Sprintf("https://github.com/machbase/neo-launcher/releases/download/%s/neo-launcher-%s-darwin.zip",
			version, version)
	} else {
		url = fmt.Sprintf("https://github.com/machbase/neo-launcher/releases/download/%s/neo-launcher-%s-%s-%s.zip",
			version, version, runtime.GOOS, runtime.GOARCH)
	}
	dst := "./tmp/neow.zip"
	if runtime.GOOS == "windows" {
		if err := wget(url, dst); err != nil {
			return err
		}
	} else {
		if err := sh.RunV("curl",
			"-fsSL",
			"--retry", "3",
			"--retry-delay", "5",
			"--connect-timeout", "10",
			"-o", dst,
			"-L", url); err != nil {
			return err
		}
	}
	if err := unzip(dst, "./tmp"); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		os.Rename("./tmp/neo-launcher.exe", "./tmp/neow.exe")
	case "darwin":
		os.Rename("./tmp/neo-launcher.app", "./tmp/neow.app")
	}
	return nil
}

//go:embed neo-web-version.txt
var neo_web_version string

func InstallNeoWeb() error {
	return InstallNeoWebX(neo_web_version)
}

func InstallNeoWebX(ver string) error {
	return installArtifact(webArtifact(ver), true)
}

//go:embed neo-engine-version.txt
var neo_engine_version string

func InstallNeoEngine() error {
	return InstallNeoEngineX(neo_engine_version)
}

func InstallNeoEngineX(ver string) error {
	return installArtifact(engineArtifact(ver, runtime.GOOS, runtime.GOARCH), true)
}

type releaseArtifact struct {
	version, name, url, destination, required string
	web                                       bool
}

func webArtifact(ver string) releaseArtifact {
	ver = strings.TrimSpace(ver)
	return releaseArtifact{
		version: ver, name: "web-ui.zip",
		url:         fmt.Sprintf("https://github.com/machbase/neo-web/releases/download/%s/web-ui.zip", ver),
		destination: "./mods/server/web/ui", required: "index.html", web: true,
	}
}

func engineArtifact(ver, archiveOS, archiveArch string) releaseArtifact {
	ver = strings.TrimSpace(ver)
	if archiveOS == "linux" && archiveArch == "arm" {
		archiveArch = "arm32"
	}
	archiveName := fmt.Sprintf("libmachengine_standard_%s_%s.zip", archiveOS, archiveArch)
	return releaseArtifact{
		version: ver, name: archiveName,
		url:         fmt.Sprintf("https://github.com/machbase/packages/releases/download/%s/%s", ver, archiveName),
		destination: "./spi/mach/native", required: strings.TrimSuffix(archiveName, ".zip") + ".a",
	}
}

func goTarget() (string, string) {
	targetOS, targetArch := os.Getenv("GOOS"), os.Getenv("GOARCH")
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	return targetOS, targetArch
}

func ensureBuildArtifacts(targetOS, targetArch string) error {
	if err := installArtifact(webArtifact(neo_web_version), false); err != nil {
		return err
	}
	return installArtifact(engineArtifact(neo_engine_version, targetOS, targetArch), false)
}

func installArtifact(a releaseArtifact, force bool) error {
	if a.version == "" || a.version == "." || a.version == ".." ||
		strings.ContainsAny(a.version, `/\`) {
		return fmt.Errorf("invalid artifact version %q", a.version)
	}
	cacheDir := filepath.Join("tmp", "release-cache", strings.TrimSuffix(a.name, ".zip"), a.version)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	cache := filepath.Join(cacheDir, a.name)
	if !force {
		if err := validateArtifact(cache, a); err == nil {
			fmt.Printf("Using cached %s %s\n", a.name, a.version)
		} else if !os.IsNotExist(err) {
			fmt.Printf("Invalid cached %s %s: %v; downloading again\n", a.name, a.version, err)
			force = true
		} else {
			force = true
		}
	}
	if force {
		fmt.Printf("Downloading %s %s\n", a.name, a.version)
		if err := downloadArtifact(a, cache); err != nil {
			return err
		}
	}
	matches, err := artifactInstalled(cache, a)
	if err != nil {
		return err
	}
	if matches && !force {
		return nil
	}
	fmt.Printf("Installing %s %s\n", a.name, a.version)
	return extractArtifact(cache, a)
}

func downloadArtifact(a releaseArtifact, cache string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Get(a.url)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", a.url, response.Status)
	}
	file, err := os.CreateTemp(filepath.Dir(cache), ".download-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		return fmt.Errorf("download %s: %w", a.url, err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := validateArtifact(file.Name(), a); err != nil {
		return fmt.Errorf("invalid download %s: %w", a.url, err)
	}
	if err := os.Rename(file.Name(), cache); err != nil {
		return fmt.Errorf("cache %s: %w", a.name, err)
	}
	return nil
}

func validateArtifact(path string, a releaseArtifact) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	found := false
	seen := make(map[string]bool)
	for _, file := range archive.File {
		name := file.Name
		if !fs.ValidPath(strings.TrimSuffix(name, "/")) || !filepath.IsLocal(filepath.FromSlash(name)) ||
			strings.Contains(name, `\`) || seen[name] ||
			(!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
			return fmt.Errorf("invalid archive entry %q", name)
		}
		seen[name] = true
		if name == a.required && file.Mode().IsRegular() {
			found = true
		}
		if file.FileInfo().IsDir() {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		_, readErr := io.Copy(io.Discard, reader)
		closeErr := reader.Close()
		if readErr != nil {
			return fmt.Errorf("read %s: %w", name, readErr)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if !found {
		return fmt.Errorf("archive missing %s", a.required)
	}
	return nil
}

func artifactInstalled(path string, a releaseArtifact) (bool, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return false, err
	}
	defer archive.Close()
	expected := make(map[string]bool)
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || (!a.web && entry.Name != a.required) {
			continue
		}
		expected[entry.Name] = true
		installedPath := filepath.Join(a.destination, filepath.FromSlash(entry.Name))
		info, err := os.Lstat(installedPath)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, nil
		}
		file, err := os.Open(installedPath)
		if err != nil {
			return false, err
		}
		hash := crc32.NewIEEE()
		size, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if readErr != nil {
			return false, readErr
		}
		if closeErr != nil {
			return false, closeErr
		}
		if uint64(size) != entry.UncompressedSize64 || hash.Sum32() != entry.CRC32 {
			return false, nil
		}
	}
	if a.web {
		extra := false
		err := filepath.WalkDir(a.destination, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				rel, err := filepath.Rel(a.destination, path)
				if err != nil {
					return err
				}
				if !expected[filepath.ToSlash(rel)] {
					extra = true
					return fs.SkipAll
				}
			}
			return nil
		})
		if err != nil {
			return false, err
		}
		return !extra, nil
	}
	return true, nil
}

func extractArtifact(path string, a releaseArtifact) error {
	stage, err := os.MkdirTemp("tmp", "install-*")
	if err != nil {
		return err
	}
	defer func() {
		if stage != "" {
			os.RemoveAll(stage)
		}
	}()
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	output := filepath.Join(stage, "output")
	if err := os.Mkdir(output, 0755); err != nil {
		return err
	}
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || (!a.web && entry.Name != a.required) {
			continue
		}
		dst := filepath.Join(output, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		reader, err := entry.Open()
		if err != nil {
			return err
		}
		writer, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm())
		if err != nil {
			reader.Close()
			return err
		}
		_, copyErr := io.Copy(writer, reader)
		readCloseErr, writeCloseErr := reader.Close(), writer.Close()
		if copyErr != nil {
			return copyErr
		}
		if readCloseErr != nil {
			return readCloseErr
		}
		if writeCloseErr != nil {
			return writeCloseErr
		}
	}
	if !a.web {
		if err := os.MkdirAll(a.destination, 0755); err != nil {
			return err
		}
		return os.Rename(filepath.Join(output, a.required), filepath.Join(a.destination, a.required))
	}
	backup := filepath.Join(stage, "previous")
	hadPrevious := false
	if err := os.Rename(a.destination, backup); err == nil {
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(output, a.destination); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backup, a.destination); restoreErr != nil {
				// Keep the previous UI if the rollback fails.
				stage = ""
				return fmt.Errorf("install UI: %v; restore previous UI at %s: %w", err, backup, restoreErr)
			}
		}
		return err
	}
	return nil
}

func wget(url string, dst string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	rsp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	dlfn := func(in io.Reader) error {
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		if len, err := io.Copy(out, in); err != nil {
			return err
		} else {
			fmt.Println("download:", len)
		}
		return out.Close()
	}

	if rsp.StatusCode == 302 {
		req, err = http.NewRequest("GET", rsp.Header.Get("Location"), nil)
		if err != nil {
			return err
		}
		rsp, err = client.Do(req)
		if err != nil {
			return err
		}
		if err := dlfn(rsp.Body); err != nil {
			return err
		}
	} else {
		if err := dlfn(rsp.Body); err != nil {
			return err
		}
	}
	return nil
}

func unzip(source, destination string) error {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, file := range archive.Reader.File {
		reader, err := file.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		path := filepath.Join(destination, file.Name)
		// Remove file if it already exists; no problem if it doesn't; other cases can error out below
		_ = os.Remove(path)
		// Create a directory at path, including parents
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
		// If file is _supposed_ to be a directory, we're done
		if file.FileInfo().IsDir() {
			continue
		}
		// otherwise, remove that directory (_not_ including parents)
		err = os.Remove(path)
		if err != nil {
			return err
		}
		// and create the actual file.  This ensures that the parent directories exist!
		// An archive may have a single file with a nested path, rather than a file for each parent dir
		writer, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer writer.Close()
		_, err = io.Copy(writer, reader)
		if err != nil {
			return err
		}
	}
	return nil
}

func Examples() error {
	return Example("")
}

func Example(name string) error {
	excludes := map[string]bool{
		"grpc_wave":       true,
		"http_wave":       true,
		"mqtt_subscriber": true,
	}
	examplesGoPath := `examples/go`
	if name == "" {
		entries, err := os.ReadDir(examplesGoPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if _, ok := excludes[name]; ok {
				continue
			}
			path := filepath.Join(examplesGoPath, name, name+".go")
			if err := runGoExample(path); err != nil {
				fmt.Println("ERROR", path)
				fmt.Println(err.Error())
			}
		}
	} else {
		path := filepath.Join(examplesGoPath, name, name+".go")
		if err := runGoExample(path); err != nil {
			fmt.Println("ERROR", path)
			fmt.Println(err.Error())
		}
	}
	return nil
}

func runGoExample(path string) error {
	fmt.Println("--------")
	fmt.Println("EXAMPLE:", path)
	env := map[string]string{}
	if err := sh.RunWith(env, "go", "run", path); err != nil {
		return err
	}
	return nil
}

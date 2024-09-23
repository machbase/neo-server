package main

import (
	"archive/zip"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	_ "github.com/magefile/mage/mage"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = BuildNeoServer

var Aliases = map[string]any{
	"machbase-neo":    BuildNeoServer,
	"neoshell":        BuildNeoShell,
	"cleanpackage":    CleanPackage,
	"buildversion":    BuildVersion,
	"install-neo-web": InstallNeoWeb,
}

var vLastVersion string
var vLastCommit string
var vIsNightly bool
var vBuildVersion string

func BuildVersion() {
	mg.Deps(GetVersion)
	fmt.Println(vBuildVersion)
}

func BuildNeoServer() error {
	return Build("machbase-neo")
}

func BuildNeoShell() error {
	return Build("neoshell")
}

func Build(target string) error {
	mg.Deps(CheckTmp, GetVersion)

	fmt.Println("Build", target, vBuildVersion, "...")

	mod := "github.com/machbase/neo-server"
	edition := "standard"
	timestamp := time.Now().Format("2006-01-02T15:04:05")
	gitSHA := vLastCommit[0:8]
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	env := map[string]string{"GO111MODULE": "on"}
	if target == "neoshell" {
		// FIXME: neoshell should not link to engine
		env["CGO_ENABLED"] = "1"
	}

	args := []string{"build"}
	ldflags := strings.Join([]string{
		"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
		"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
		"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
		"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
		"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
		// "-s", // this may reduce binary size about (110M -> 86M)
	}, " ")
	args = append(args, "-ldflags", ldflags)

	// executable file
	if runtime.GOOS == "windows" {
		args = append(args, "-tags=timetzdata")
		args = append(args, "-o", fmt.Sprintf("./tmp/%s.exe", target))
	} else {
		args = append(args, "-o", fmt.Sprintf("./tmp/%s", target))
	}
	// source directory
	args = append(args, fmt.Sprintf("./main/%s", target))

	if err := sh.RunV("go", "mod", "tidy"); err != nil {
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
	fmt.Println("Build", target, vBuildVersion, "...")

	zigVer, err := sh.Output("zig", "version")
	if err != nil {
		fmt.Println("error: Zig is not installed. Please download and follow installation")
		fmt.Println("instructions at https://ziglang.org/ to continue.")
		return err
	}
	fmt.Println("zig", zigVer)

	mod := "github.com/machbase/neo-server"
	edition := "standard"
	timestamp := time.Now().Format("2006-01-02T15:04:05")
	gitSHA := vLastCommit[0:8]
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	env := map[string]string{"GO111MODULE": "on"}
	if target != "neoshell" {
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
	ldflags := strings.Join([]string{
		"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
		"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
		"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
		"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
		"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
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
	args = append(args, fmt.Sprintf("./main/%s", target))

	if err := sh.RunV("go", "mod", "tidy"); err != nil {
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
	mg.Deps(CheckTmp)

	env := map[string]string{
		"GO111MODULE": "on",
		"CGO_ENABLED": "1",
	}

	if runtime.GOOS == "linux" {
		env["CGO_LDFLAGS"] = "-pthread"
	}
	if err := sh.RunWithV(env, "go", "mod", "tidy"); err != nil {
		return err
	}

	testArgs := []string{
		"test", "-cover", "-coverprofile", "./tmp/cover.out",
		"./booter/...",
		"./mods/...",
		"./api/...",
	}

	if runtime.GOOS != "windows" {
		testArgs = append(testArgs, "./test/...")
	}

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

func CheckMoq() error {
	const moqRepo = "github.com/matryer/moq@latest"
	if _, err := sh.Output("moq", "-version"); err != nil {
		err = sh.RunV("go", "install", moqRepo)
		if err != nil {
			return err
		}
	}
	return nil
}

func Generate() error {
	mg.Deps(CheckMoq)
	return sh.RunV("go", "generate", "./...")
}

func Protoc() error {
	args := []string{}
	if len(args) == 0 {
		args = []string{
			"mgmt", "bridge", "schedule",
		}
	}

	for _, mod := range args {
		fmt.Printf("protoc regen api/proto/%s.proto...\n", mod)
		sh.RunV("protoc", "-I", "api/proto", mod+".proto",
			"--experimental_allow_proto3_optional",
			fmt.Sprintf("--go_out=./api/%s", mod), "--go_opt=paths=source_relative",
			fmt.Sprintf("--go-grpc_out=./api/%s", mod), "--go-grpc_opt=paths=source_relative",
		)
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

	if targetOS == "windows" {
		if err := os.Rename(filepath.Join("tmp", "machbase-neo.exe"), filepath.Join("packages", bdir, "machbase-neo.exe")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join("tmp", "neow.exe"), filepath.Join("packages", bdir, "neow.exe")); err != nil {
			return err
		}
	} else if targetOS == "darwin" {
		if err := os.Rename(filepath.Join("tmp", "machbase-neo"), filepath.Join("packages", bdir, "machbase-neo")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join("tmp", "neow.app"), filepath.Join("packages", bdir, "neow.app")); err != nil {
			return err
		}
	} else {
		if err := os.Rename("./tmp/machbase-neo", filepath.Join("./packages", bdir, "machbase-neo")); err != nil {
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

	var lastTag *object.Tag
	iter, err := repo.TagObjects()
	if err != nil {
		return err
	}
	iter.ForEach(func(tagObj *object.Tag) error {
		if !strings.HasPrefix(tagObj.Name, "v") {
			return nil
		}
		if lastTag == nil {
			lastTag = tagObj
		} else {
			lastCommit, _ := lastTag.Commit()
			tagCommit, _ := tagObj.Commit()
			if tagCommit.Author.When.Sub(lastCommit.Author.When) > 0 {
				lastTag = tagObj
			}
		}
		return nil
	})

	lastTagCommit, err := lastTag.Commit()
	if err != nil {
		return err
	}
	vLastVersion = lastTag.Name
	vLastCommit = headCommit.Hash.String()
	vIsNightly = lastTagCommit.Hash.String() != vLastCommit
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

//go:embed neo-launcher-version.txt
var neo_launcher_version string

func InstallNeoLauncher() error {
	return InstallNeoLauncherX(neo_launcher_version)
}

func InstallNeoLauncherX(version string) error {
	mg.Deps(CheckTmp)

	url := fmt.Sprintf("https://github.com/machbase/neo-launcher/releases/download/%s/neo-launcher-%s-%s-amd64.zip",
		version, version, runtime.GOOS)
	dst := "./tmp/neow.zip"
	if runtime.GOOS == "windows" {
		if err := wget(url, dst); err != nil {
			return err
		}
	} else {
		if err := sh.RunV("wget", "-O", dst, "-L", url); err != nil {
			return err
		}
	}
	if err := unzip(dst, "./tmp"); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		os.Rename("./tmp/neo-launcher.exe", "./tmp/neow.exe")
	} else if runtime.GOOS == "darwin" {
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
	mg.Deps(CheckTmp)

	url := fmt.Sprintf("https://github.com/machbase/neo-web/releases/download/%s/web-ui.zip", ver)
	dst := "./tmp/web-ui.zip"
	uiDir := "./mods/service/httpd/web/ui"

	if runtime.GOOS == "windows" {
		if err := wget(url, dst); err != nil {
			return err
		}
	} else {
		if err := sh.RunV("wget", "-O", dst, "-L", url); err != nil {
			return err
		}
	}

	// remove web/ui/
	os.RemoveAll(uiDir)
	// create web/ui/
	if err := os.Mkdir(uiDir, 0755); err != nil {
		return err
	}
	if err := unzip(dst, uiDir); err != nil {
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

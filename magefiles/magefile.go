package main

import (
	"archive/zip"
	"fmt"
	"io"
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

var Default = Build

var Aliases = map[string]any{
	"machbase-neo":         Build,
	"neow":                 BuildNeow,
	"neoshell":             BuildNeoShell,
	"package-machbase-neo": Package,
	"package-neow":         PackageNeow,
	"package-neoshell":     PackageNeoShell,
	"cleanpackage":         CleanPackage,
	"buildversion":         BuildVersion,
}

var vLastVersion string
var vLastCommit string
var vIsNightly bool
var vBuildVersion string

func BuildVersion() {
	mg.Deps(GetVersion)
	fmt.Println(vBuildVersion)
}

func Build() error {
	mg.Deps(CheckTmp, GetVersion)
	return build("machbase-neo")
}

func BuildNeow() error {
	mg.Deps(GetVersion, CheckTmp, CheckFyne)
	return buildNeoW()
}

func BuildNeoShell() error {
	mg.Deps(CheckTmp, GetVersion)
	return build("neoshell")
}

func build(target string) error {
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
	} else {
		env["CGO_ENABLED"] = "1"
	}

	args := []string{"build"}
	if target != "neow" {
		ldflags := strings.Join([]string{
			"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
			"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
			"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
			"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
			"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
		}, " ")
		args = append(args, "-ldflags", ldflags)
	}
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

func buildNeoW() error {
	fmt.Println("Build", "neow", vBuildVersion, "...")
	env := map[string]string{
		"GO111MODULE": "on",
		"CGO_ENABLED": "0",
	}
	appIcon, err := filepath.Abs(filepath.Join(".", "main", "neow", "res", "appicon.png"))
	if err != nil {
		return err
	}
	args := []string{
		"package",
		"--os", runtime.GOOS,
		"--src", filepath.Join(".", "main", "neow"),
		"--icon", appIcon,
		"--id", "com.machbase.neow",
	}
	if err := sh.RunWithV(env, "fyne", args...); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		os.Rename("./main/neow/neow.exe", "./tmp/neow.exe")
	} else if runtime.GOOS == "darwin" {
		os.RemoveAll("./tmp/neow.app")
		if err := os.Rename("neow.app", "./tmp/neow.app"); err != nil {
			return err
		}
		if err := build("machbase-neo"); err != nil {
			return err
		}
		if err := os.Rename("./tmp/machbase-neo", "./tmp/neow.app/Contents/MacOS/machbase-neo"); err != nil {
			return err
		}
	} else {
		if err := os.Rename("./main/neow/neow", "./tmp/neow"); err != nil {
			return err
		}
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
			return fmt.Errorf("error: unsupproted linux/%s", targetArch)
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
			return fmt.Errorf("error: unsupproted darwin/%s", targetArch)
		}
	case "windows":
		env["CC"] = "zig cc -target x86_64-windows-none"
		env["CXX"] = "zig c++ -target x86_64-windows-none"
	default:
		return fmt.Errorf("error: unsupproted os %s", targetOS)
	}

	args := []string{"build"}
	if target != "neow" {
		ldflags := strings.Join([]string{
			"-X", fmt.Sprintf("%s/mods.goVersionString=%s", mod, goVersion),
			"-X", fmt.Sprintf("%s/mods.versionString=%s", mod, vBuildVersion),
			"-X", fmt.Sprintf("%s/mods.versionGitSHA=%s", mod, gitSHA),
			"-X", fmt.Sprintf("%s/mods.editionString=%s", mod, edition),
			"-X", fmt.Sprintf("%s/mods.buildTimestamp=%s", mod, timestamp),
		}, " ")
		args = append(args, "-ldflags", ldflags)
	}
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
		env["CGO_LDFLAGS"] = "-lpthread -ljemalloc"
	}
	if err := sh.RunWithV(env, "go", "mod", "tidy"); err != nil {
		return err
	}

	if err := sh.RunWithV(env, "go", "test", "-cover", "-coverprofile", "./tmp/cover.out",
		"./booter/...",
		"./mods/...",
		"./api/...",
	); err != nil {
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

func CheckFyne() error {
	const fyneVersion = "v2.3.5"
	const fyneRepo = "fyne.io/fyne/v2/cmd/fyne@latest"
	if verout, err := sh.Output("fyne", "--version"); err != nil {
		err = sh.RunV("go", "install", fyneRepo)
		if err != nil {
			return err
		}
	} else {
		// fyne version v2.3.5
		tok := strings.Fields(verout)
		if len(tok) != 3 {
			return fmt.Errorf("invalid fyne verison: %s", verout)
		}
		ver, err := semver.NewVersion(tok[2])
		if err != nil {
			return err
		}
		expectedFyneVer, _ := semver.NewVersion(fyneVersion)
		if ver.Compare(expectedFyneVer) < 0 {
			err = sh.RunV("go", "install", fyneRepo)
			if err != nil {
				return err
			}
		}
	}
	return nil
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
			"machrpc", "mgmt", "bridge", "schedule",
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
	return package0("machbase-neo")
}

func PackageNeow() error {
	return package0("neow")
}

func PackageNeoShell() error {
	return package0("neoshell")
}

func package0(target string) error {
	return PackageX(target, runtime.GOOS, runtime.GOARCH)
}

func PackageX(target string, targetOS string, targetArch string) error {
	mg.Deps(CleanPackage, GetVersion, CheckTmp)
	bdir := fmt.Sprintf("%s-%s-%s-%s", target, vBuildVersion, targetOS, targetArch)
	if targetArch == "arm" {
		bdir = fmt.Sprintf("%s-%s-%s-arm32", target, vBuildVersion, targetOS)
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
	} else if targetOS == "darwin" && target == "neow" {
		err := os.Rename("./tmp/neow.app", filepath.Join("./packages", bdir, "neow.app"))
		if err != nil {
			return err
		}
	} else {
		err := os.Rename("./tmp/machbase-neo", filepath.Join("./packages", bdir, "machbase-neo"))
		if err != nil {
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

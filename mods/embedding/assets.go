package embedding

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const modelRevision = "5617a9f61b028005a4858fdac845db406aefb181"

type asset struct {
	path string
	url  string
	hash string
}

var modelAssets = []asset{
	{
		path: "onnx/model.onnx",
		url:  "https://huggingface.co/BAAI/bge-m3/resolve/" + modelRevision + "/onnx/model.onnx",
		hash: "f84251230831afb359ab26d9fd37d5936d4d9bb5d1d5410e66442f630f24435b",
	},
	{
		path: "onnx/model.onnx_data",
		url:  "https://huggingface.co/BAAI/bge-m3/resolve/" + modelRevision + "/onnx/model.onnx_data",
		hash: "1eebfb28493f67bba03ce0ef64bfdc7fc5a3bd9d7493f818bb1d78cd798416b4",
	},
	{
		path: "tokenizer.json",
		url:  "https://huggingface.co/BAAI/bge-m3/resolve/" + modelRevision + "/tokenizer.json",
		hash: "21106b6d7dab2952c1d496fb21d5dc9db75c28ed361a05f5020bbba27810dd08",
	},
}

const runtimeURL = "https://github.com/microsoft/onnxruntime/releases/download/v1.29.0/onnxruntime-linux-x64-1.29.0.tgz"
const runtimeHash = "c3fddc4f139a045b0c4902c57410f0694f1c2fdf9b6939fbe38b1aeae7cd14ba"
const runtimeLibraryName = "libonnxruntime.so.1.29.0"
const runtimeLibraryHash = "5715f06d8992ca8eeeddcce43df3a7d38f97d537052126f558e912cb312460ca"

func ensureAssets(ctx context.Context, config Config) (string, string, error) {
	release, err := lockAssets(ctx, config.CacheDir)
	if err != nil {
		return "", "", err
	}
	defer release()
	modelDir := filepath.Join(config.CacheDir, "bge-m3", modelRevision)
	for _, file := range modelAssets {
		if err := ensureFile(ctx, filepath.Join(modelDir, file.path), file.url, file.hash); err != nil {
			return "", "", err
		}
	}
	if config.RuntimeLibrary != "" {
		if stat, err := os.Stat(config.RuntimeLibrary); err != nil || stat.IsDir() {
			return "", "", fmt.Errorf("ONNX Runtime library not found: %s", config.RuntimeLibrary)
		}
		return modelDir, config.RuntimeLibrary, nil
	}
	runtimeDir := filepath.Join(config.CacheDir, "onnxruntime", "1.29.0")
	archive := filepath.Join(runtimeDir, "onnxruntime-linux-x64-1.29.0.tgz")
	if err := ensureFile(ctx, archive, runtimeURL, runtimeHash); err != nil {
		return "", "", err
	}
	library := filepath.Join(runtimeDir, runtimeLibraryName)
	if ok, err := verifyFile(library, runtimeLibraryHash); err != nil || !ok {
		if err := extractRuntimeLibrary(archive, library); err != nil {
			return "", "", err
		}
	}
	return modelDir, library, nil
}

func ensureFile(ctx context.Context, path, url, expectedHash string) error {
	if ok, err := verifyFile(path, expectedHash); err == nil && ok {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", path, response.StatusCode)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".embedding-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temporary, digest), response.Body); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if hex.EncodeToString(digest.Sum(nil)) != expectedHash {
		return fmt.Errorf("embedding asset SHA-256 mismatch: %s", path)
	}
	return os.Rename(temporary.Name(), path)
}

func verifyFile(path, expectedHash string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return false, err
	}
	return hex.EncodeToString(digest.Sum(nil)) == expectedHash, nil
}

func extractRuntimeLibrary(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, "/lib/"+runtimeLibraryName) {
			continue
		}
		temporary, err := os.CreateTemp(filepath.Dir(destination), ".onnxruntime-*")
		if err != nil {
			return err
		}
		defer os.Remove(temporary.Name())
		if _, err := io.Copy(temporary, reader); err != nil {
			temporary.Close()
			return err
		}
		if err := temporary.Chmod(0755); err != nil {
			temporary.Close()
			return err
		}
		if err := temporary.Sync(); err != nil {
			temporary.Close()
			return err
		}
		if err := temporary.Close(); err != nil {
			return err
		}
		return os.Rename(temporary.Name(), destination)
	}
	return fmt.Errorf("ONNX Runtime library missing from %s", archive)
}

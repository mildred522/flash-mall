package assetstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Asset struct {
	URL    string
	Path   string
	SHA256 string
	Size   int64
}

type Filesystem struct {
	root string
}

func NewFilesystem(root string) *Filesystem {
	return &Filesystem{root: filepath.Clean(strings.TrimSpace(root))}
}

func (s *Filesystem) Save(ctx context.Context, namespace, extension string, src io.Reader, maxBytes int64) (Asset, error) {
	if ctx == nil {
		return Asset{}, errors.New("asset context is not configured")
	}
	if err := ctx.Err(); err != nil {
		return Asset{}, err
	}
	if s == nil || s.root == "" || s.root == "." {
		return Asset{}, errors.New("asset root is not configured")
	}
	if err := validateNamespace(namespace); err != nil {
		return Asset{}, err
	}
	if !validExtension(extension) {
		return Asset{}, errors.New("invalid asset extension")
	}
	if src == nil || maxBytes <= 0 {
		return Asset{}, errors.New("invalid asset input")
	}

	dir := filepath.Join(s.root, filepath.FromSlash(namespace))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Asset{}, fmt.Errorf("create asset directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return Asset{}, fmt.Errorf("create asset temp file: %w", err)
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		_ = temp.Close()
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	hash := sha256.New()
	written, err := io.Copy(
		io.MultiWriter(temp, hash),
		io.LimitReader(contextReader{ctx: ctx, reader: src}, maxBytes+1),
	)
	if err != nil {
		return Asset{}, fmt.Errorf("write asset: %w", err)
	}
	if written > maxBytes {
		return Asset{}, fmt.Errorf("asset exceeds %d bytes", maxBytes)
	}
	if err = temp.Sync(); err != nil {
		return Asset{}, fmt.Errorf("sync asset: %w", err)
	}
	if err = temp.Close(); err != nil {
		return Asset{}, fmt.Errorf("close asset: %w", err)
	}

	digest := hex.EncodeToString(hash.Sum(nil))
	name := digest + strings.ToLower(extension)
	finalPath := filepath.Join(dir, name)
	if _, statErr := os.Stat(finalPath); statErr == nil {
		valid, verifyErr := fileMatchesSHA256(finalPath, digest)
		if verifyErr != nil {
			return Asset{}, fmt.Errorf("verify existing asset: %w", verifyErr)
		}
		if valid {
			return Asset{URL: assetURL(namespace, name), Path: finalPath, SHA256: digest, Size: written}, nil
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Asset{}, fmt.Errorf("inspect asset: %w", statErr)
	}
	if err = os.Rename(tempPath, finalPath); err != nil {
		return Asset{}, fmt.Errorf("commit asset: %w", err)
	}
	keepTemp = false
	if err = os.Chmod(finalPath, 0o644); err != nil {
		return Asset{}, fmt.Errorf("set asset permissions: %w", err)
	}
	if err = syncDirectory(dir); err != nil {
		return Asset{}, fmt.Errorf("sync asset directory: %w", err)
	}
	return Asset{URL: assetURL(namespace, name), Path: finalPath, SHA256: digest, Size: written}, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func fileMatchesSHA256(filename, expected string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected), nil
}

func syncDirectory(dirname string) error {
	dir, err := os.Open(dirname)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

func validateNamespace(namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || strings.Contains(namespace, "\\") || path.IsAbs(namespace) {
		return errors.New("invalid asset namespace")
	}
	for _, part := range strings.Split(namespace, "/") {
		if part == "" || part == "." || part == ".." || path.Base(part) != part {
			return errors.New("invalid asset namespace")
		}
	}
	return nil
}

func validExtension(extension string) bool {
	if len(extension) < 2 || len(extension) > 8 || extension[0] != '.' {
		return false
	}
	for _, char := range extension[1:] {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func assetURL(namespace, name string) string {
	return "/uploads/" + path.Join(namespace, name)
}

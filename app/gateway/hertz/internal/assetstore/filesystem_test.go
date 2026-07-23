package assetstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemSaveUsesContentAddressAndSurvivesStoreRecreation(t *testing.T) {
	root := t.TempDir()
	content := []byte("durable-image-content")

	first, err := NewFilesystem(root).Save(context.Background(), "products", ".png", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("save first asset: %v", err)
	}
	second, err := NewFilesystem(root).Save(context.Background(), "products", ".png", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("save duplicate asset: %v", err)
	}

	if first.URL != second.URL || first.SHA256 != second.SHA256 {
		t.Fatalf("content address changed: first=%#v second=%#v", first, second)
	}
	if !strings.HasPrefix(first.URL, "/uploads/products/") {
		t.Fatalf("unexpected public URL: %s", first.URL)
	}
	got, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatalf("read saved asset: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("saved content=%q, want %q", got, content)
	}
}

func TestFilesystemSaveRejectsEscapingNamespaceAndOversizedInput(t *testing.T) {
	store := NewFilesystem(t.TempDir())
	if _, err := store.Save(context.Background(), "../outside", ".png", strings.NewReader("image"), 5); err == nil {
		t.Fatal("expected escaping namespace to fail")
	}
	if _, err := store.Save(context.Background(), "products", ".png", strings.NewReader("too-large"), 3); err == nil {
		t.Fatal("expected oversized input to fail")
	}
}

func TestFilesystemReplacesCorruptContentAddressedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := []byte("durable image")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	dir := filepath.Join(root, "products")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, digest+".png")
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	asset, err := NewFilesystem(root).Save(context.Background(), "products", ".png", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(asset.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("corrupt destination was reused: got %q", got)
	}
}

func TestFilesystemSaveHonorsCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewFilesystem(t.TempDir()).Save(
		ctx,
		"products",
		".png",
		strings.NewReader("image"),
		5,
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save() error=%v, want context canceled", err)
	}
}

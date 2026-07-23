package assetstore

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestFilesystemSaveUsesContentAddressAndSurvivesStoreRecreation(t *testing.T) {
	root := t.TempDir()
	content := []byte("durable-image-content")

	first, err := NewFilesystem(root).Save("products", ".png", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("save first asset: %v", err)
	}
	second, err := NewFilesystem(root).Save("products", ".png", bytes.NewReader(content), int64(len(content)))
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
	if _, err := store.Save("../outside", ".png", strings.NewReader("image"), 5); err == nil {
		t.Fatal("expected escaping namespace to fail")
	}
	if _, err := store.Save("products", ".png", strings.NewReader("too-large"), 3); err == nil {
		t.Fatal("expected oversized input to fail")
	}
}

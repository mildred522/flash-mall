package handler

import (
	"path/filepath"
	"testing"
)

func TestStaticAssetPathStaysWithinRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "app", "web", "products")
	got, err := staticAssetPath(root, "/100.svg")
	if err != nil {
		t.Fatalf("expected valid asset path: %v", err)
	}
	want := filepath.Join(root, "100.svg")
	if got != want {
		t.Fatalf("asset path = %q, want %q", got, want)
	}
}

func TestStaticAssetPathRejectsTraversal(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "app", "web", "products")
	for _, raw := range []string{"../admin.html", "/../../admin.html", "..\\admin.html"} {
		if _, err := staticAssetPath(root, raw); err == nil {
			t.Fatalf("expected traversal to be rejected: %q", raw)
		}
	}
}

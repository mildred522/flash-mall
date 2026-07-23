package config

import "testing"

func TestNormalizedUploadDirUsesStableDefault(t *testing.T) {
	t.Parallel()
	if got := (Config{}).NormalizedUploadDir(); got != ".runtime/uploads" {
		t.Fatalf("NormalizedUploadDir()=%q", got)
	}
	if got := (Config{UploadDir: " /data/uploads "}).NormalizedUploadDir(); got != "/data/uploads" {
		t.Fatalf("NormalizedUploadDir()=%q", got)
	}
}

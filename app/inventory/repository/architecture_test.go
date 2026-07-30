package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryDoesNotExecuteDDL(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read repository directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Clean(entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		upper := strings.ToUpper(string(content))
		if strings.Contains(upper, "CREATE TABLE") || strings.Contains(upper, "ALTER TABLE") {
			t.Errorf("%s contains runtime DDL; schema changes belong in scripts/k8s/schema.sql modules", entry.Name())
		}
	}
}

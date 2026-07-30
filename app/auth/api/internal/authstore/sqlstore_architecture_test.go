package authstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLStoreDoesNotSeedDemoAccountsAtRuntime(t *testing.T) {
	files, err := filepath.Glob("sqlstore*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"ensureDemoUser", "13800000002", "admin123"} {
			if strings.Contains(string(content), forbidden) {
				t.Fatalf("%s contains runtime demo seed token %q", file, forbidden)
			}
		}
	}
}

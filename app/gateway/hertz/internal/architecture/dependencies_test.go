package architecture_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestInventoryGeneratedCodeIsIsolatedInKitexAdapter(t *testing.T) {
	t.Parallel()

	internalRoot := internalPackageRoot(t)
	var violations []string
	err := filepath.WalkDir(internalRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(internalRoot, path)
		if err != nil {
			return err
		}
		directory := filepath.ToSlash(filepath.Dir(relative))
		if directory == "architecture" || directory == "adapters/inventorykitex" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), "app/inventory/kitex/kitex_gen/") {
			violations = append(violations, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan Hertz internal packages: %v", err)
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("inventory generated imports must stay in adapters/inventorykitex; violations: %s", strings.Join(violations, ", "))
	}
}

func TestHertzRuntimeDoesNotExecuteDDL(t *testing.T) {
	t.Parallel()

	internalRoot := internalPackageRoot(t)
	violations := filesContaining(t, internalRoot, func(relative string, content string) bool {
		return !strings.HasSuffix(relative, "_test.go") &&
			(strings.Contains(strings.ToUpper(content), "CREATE TABLE") || strings.Contains(strings.ToUpper(content), "ALTER TABLE"))
	})
	if len(violations) > 0 {
		t.Fatalf("runtime DDL is forbidden in Hertz; move schema changes to deploy/mysql/init-db.sql: %s", strings.Join(violations, ", "))
	}
}

func TestOrderTableWritesAreIsolatedInLegacyAdapter(t *testing.T) {
	t.Parallel()

	internalRoot := internalPackageRoot(t)
	violations := filesContaining(t, internalRoot, func(relative string, content string) bool {
		if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "adapters/legacyorder/") {
			return false
		}
		upper := strings.ToUpper(content)
		return strings.Contains(upper, "UPDATE ORDERS") || strings.Contains(upper, "INSERT INTO ORDERS") || strings.Contains(upper, "DELETE FROM ORDERS")
	})
	if len(violations) > 0 {
		t.Fatalf("order table writes must stay in adapters/legacyorder until order-rpc owns them: %s", strings.Join(violations, ", "))
	}
}

func TestPortsAreFrameworkAndInfrastructureNeutral(t *testing.T) {
	t.Parallel()
	root := filepath.Join(internalPackageRoot(t), "ports")
	forbidden := []string{"database/sql", "cloudwego", "kitex_gen", "go-zero", "sqlx", "zrpc"}
	violations := filesContaining(t, root, func(relative string, content string) bool {
		for _, dependency := range forbidden {
			if strings.Contains(content, dependency) {
				return true
			}
		}
		return false
	})
	if len(violations) > 0 {
		t.Fatalf("ports must contain business contracts only: %s", strings.Join(violations, ", "))
	}
}

func TestTransportDoesNotImportPersistenceOrRPCClients(t *testing.T) {
	t.Parallel()
	root := filepath.Join(internalPackageRoot(t), "transport")
	forbidden := []string{"database/sql", "kitex_gen", "sqlx", "zrpc", "orderclient", "productclient"}
	violations := filesContaining(t, root, func(relative string, content string) bool {
		for _, dependency := range forbidden {
			if strings.Contains(content, dependency) {
				return true
			}
		}
		return false
	})
	if len(violations) > 0 {
		t.Fatalf("transport may depend on HTTP framework and ports, not persistence or RPC clients: %s", strings.Join(violations, ", "))
	}
}

func filesContaining(t *testing.T, root string, matches func(relative string, content string) bool) []string {
	t.Helper()
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(relative, "architecture/") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if matches(relative, string(content)) {
			violations = append(violations, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan Hertz internal packages: %v", err)
	}
	sort.Strings(violations)
	return violations
}

func internalPackageRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test location")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

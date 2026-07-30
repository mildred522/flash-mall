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
		t.Fatalf("runtime DDL is forbidden in Hertz; move schema changes to scripts/k8s/schema.sql modules: %s", strings.Join(violations, ", "))
	}
}

func TestHertzDoesNotWriteOrderTables(t *testing.T) {
	t.Parallel()

	internalRoot := internalPackageRoot(t)
	violations := filesContaining(t, internalRoot, func(relative string, content string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		upper := strings.ToUpper(content)
		return strings.Contains(upper, "UPDATE ORDERS") || strings.Contains(upper, "INSERT INTO ORDERS") ||
			strings.Contains(upper, "DELETE FROM ORDERS") || strings.Contains(upper, "INSERT INTO ORDER_STATUS_LOG")
	})
	if len(violations) > 0 {
		t.Fatalf("Hertz must delegate order writes to order-rpc: %s", strings.Join(violations, ", "))
	}
}

func TestLegacyOrderAdapterWasRemoved(t *testing.T) {
	t.Parallel()
	legacyPath := filepath.Join(internalPackageRoot(t), "adapters", "legacyorder", "command.go")
	if _, err := os.Stat(legacyPath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("temporary legacy order adapter source must not exist after P1: %s", legacyPath)
	}
}

func TestHertzDoesNotDependOnLegacyEntryAPI(t *testing.T) {
	t.Parallel()

	internalRoot := internalPackageRoot(t)
	violations := filesContaining(t, internalRoot, func(relative string, content string) bool {
		return strings.Contains(content, "app/entry/api")
	})
	if len(violations) > 0 {
		t.Fatalf("Hertz must not depend on the frozen Go-zero entry baseline: %s", strings.Join(violations, ", "))
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

func TestHandlersDoNotOwnPersistence(t *testing.T) {
	t.Parallel()
	root := filepath.Join(internalPackageRoot(t), "handler")
	forbidden := []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("}
	violations := filesContaining(t, root, func(relative string, content string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		for _, fragment := range forbidden {
			if strings.Contains(content, fragment) {
				return true
			}
		}
		return false
	})
	if len(violations) > 0 {
		t.Fatalf("HTTP handlers must delegate persistence to application services and adapters: %s", strings.Join(violations, ", "))
	}
}

func TestPromotionHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_promotion.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read promotion handler: %v", err)
	}
	forbidden := []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext("}
	for _, fragment := range forbidden {
		if strings.Contains(string(content), fragment) {
			t.Fatalf("promotion handler must delegate persistence and business orchestration; found %q", fragment)
		}
	}
}

func TestShowcaseHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"showcase.go", "showcase_recommendation.go"} {
		path := filepath.Join(internalPackageRoot(t), "handler", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read showcase handler %s: %v", name, err)
		}
		forbidden := []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("}
		for _, fragment := range forbidden {
			if strings.Contains(string(content), fragment) {
				t.Fatalf("%s must delegate persistence and publish rules; found %q", name, fragment)
			}
		}
	}
}

func TestCustomerOrderHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "order.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read customer order handler: %v", err)
	}
	forbidden := []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext("}
	for _, fragment := range forbidden {
		if strings.Contains(string(content), fragment) {
			t.Fatalf("customer order HTTP flow must use the order read service; found %q", fragment)
		}
	}
}

func TestBackofficeOrderHandlersDoNotOwnPersistence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"admin_order.go", "merchant_order.go"} {
		path := filepath.Join(internalPackageRoot(t), "handler", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		forbidden := []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext("}
		for _, fragment := range forbidden {
			if strings.Contains(string(content), fragment) {
				t.Errorf("%s must use the backoffice order query service; found %q", name, fragment)
			}
		}
	}
}

func TestCatalogHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "catalog.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read catalog handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("catalog HTTP flow must use catalogquery; found %q", fragment)
		}
	}
}

func TestStorefrontHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "storefront.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read storefront handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("storefront HTTP flow must use catalogquery; found %q", fragment)
		}
	}
}

func TestAdminProductQueryDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_product_query.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin product query: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("admin product query must use catalogquery; found %q", fragment)
		}
	}
}

func TestAdminSupplierHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_supplier.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin supplier handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("admin supplier flow must use the supplier application service; found %q", fragment)
		}
	}
}

func TestMerchantProfileHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "merchant_profile.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read merchant profile handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("merchant profile reads must use merchantquery; found %q", fragment)
		}
	}
}

func TestProductWriteHandlersDoNotOwnPersistence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"admin_product_mutation.go", "merchant_product_write.go"} {
		path := filepath.Join(internalPackageRoot(t), "handler", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read product write handler %s: %v", name, err)
		}
		for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
			if strings.Contains(string(content), fragment) {
				t.Errorf("%s must use productcommand; found %q", name, fragment)
			}
		}
	}
}

func TestMerchantOnboardingHandlersDoNotOwnPersistence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"admin_merchant_apply.go", "merchant_apply_create.go"} {
		path := filepath.Join(internalPackageRoot(t), "handler", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read merchant onboarding handler %s: %v", name, err)
		}
		for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
			if strings.Contains(string(content), fragment) {
				t.Errorf("%s must use merchantonboarding; found %q", name, fragment)
			}
		}
	}
}

func TestAdminOpsHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_ops.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin ops handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("admin_ops.go must use adminops; found %q", fragment)
		}
	}
}

func TestAdminReconciliationHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_reconciliation.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin reconciliation handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("admin_reconciliation.go must use reconciliation service; found %q", fragment)
		}
	}
	legacyPath := filepath.Join(internalPackageRoot(t), "handler", "admin_reconciliation_persistence.go")
	if _, err := os.Stat(legacyPath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("reconciliation persistence must not remain in handler: %s", legacyPath)
	}
}

func TestUserAddressHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "user_address.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read user address handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("user_address.go must use useraddress service; found %q", fragment)
		}
	}
}

func TestMerchantStoreHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "merchant_store.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read merchant store handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("merchant_store.go must use merchantstore service; found %q", fragment)
		}
	}
}

func TestCampaignHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_campaign.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read campaign handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("admin_campaign.go must use campaign service; found %q", fragment)
		}
	}
}

func TestAdminRefundHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "admin_refund.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin refund handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("admin_refund.go must use backoffice order queries and order RPC; found %q", fragment)
		}
	}
}

func TestStockAuditHandlerDoesNotOwnPersistence(t *testing.T) {
	t.Parallel()
	path := filepath.Join(internalPackageRoot(t), "handler", "stock_change_log.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stock audit handler: %v", err)
	}
	for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
		if strings.Contains(string(content), fragment) {
			t.Errorf("stock_change_log.go must use stockaudit service; found %q", fragment)
		}
	}
}

func TestMerchantScopeAndProductHandlersDoNotOwnPersistence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"merchant_access.go", "merchant_product.go", "product_inventory_seed.go"} {
		path := filepath.Join(internalPackageRoot(t), "handler", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range []string{"database/sql", ".RawDB()", ".QueryRowContext(", ".QueryContext(", ".ExecContext(", ".BeginTx("} {
			if strings.Contains(string(content), fragment) {
				t.Errorf("%s must use merchantquery/catalogquery services; found %q", name, fragment)
			}
		}
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

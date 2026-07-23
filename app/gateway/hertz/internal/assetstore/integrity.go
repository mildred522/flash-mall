package assetstore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type IntegrityReport struct {
	Configured        bool     `json:"configured"`
	Writable          bool     `json:"writable"`
	ReferencedFiles   int      `json:"referenced_files"`
	MissingFiles      int      `json:"missing_files"`
	MissingReferences []string `json:"missing_references,omitempty"`
}

func (r IntegrityReport) Healthy() bool {
	return r.Configured && r.Writable && r.MissingFiles == 0
}

type IntegrityChecker struct {
	root      string
	productDB *sql.DB
	orderDB   *sql.DB
}

func NewIntegrityChecker(root string, productDB, orderDB *sql.DB) *IntegrityChecker {
	return &IntegrityChecker{root: filepath.Clean(root), productDB: productDB, orderDB: orderDB}
}

func (c *IntegrityChecker) Check(ctx context.Context) (IntegrityReport, error) {
	report := IntegrityReport{Configured: strings.TrimSpace(c.root) != "" && c.root != "."}
	if !report.Configured {
		return report, fmt.Errorf("upload directory is not configured")
	}
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return report, fmt.Errorf("create upload directory: %w", err)
	}
	probe, err := os.CreateTemp(c.root, ".health-*")
	if err != nil {
		return report, fmt.Errorf("upload directory is not writable: %w", err)
	}
	probeName := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		_ = os.Remove(probeName)
		return report, fmt.Errorf("close upload health probe: %w", closeErr)
	}
	if err := os.Remove(probeName); err != nil {
		return report, fmt.Errorf("remove upload health probe: %w", err)
	}
	report.Writable = true

	references, err := c.references(ctx)
	if err != nil {
		return report, err
	}
	report.ReferencedFiles = len(references)
	for _, reference := range references {
		path, ok := c.referencePath(reference)
		if !ok {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if !os.IsNotExist(err) {
				return report, fmt.Errorf("stat upload reference %q: %w", reference, err)
			}
			report.MissingReferences = append(report.MissingReferences, reference)
		}
	}
	report.MissingFiles = len(report.MissingReferences)
	return report, nil
}

func (c *IntegrityChecker) references(ctx context.Context) ([]string, error) {
	unique := make(map[string]struct{})
	if c.productDB != nil {
		if err := collectReferences(ctx, c.productDB,
			"SELECT image_url FROM product WHERE image_url LIKE '/uploads/%'", unique); err != nil {
			return nil, fmt.Errorf("query product upload references: %w", err)
		}
	}
	if c.orderDB != nil {
		if err := collectReferences(ctx, c.orderDB, `
SELECT product_image_url FROM order_price_snapshot WHERE product_image_url LIKE '/uploads/%'
UNION
SELECT logo_url FROM merchant_store_profile WHERE logo_url LIKE '/uploads/%'
UNION
SELECT banner_url FROM merchant_store_profile WHERE banner_url LIKE '/uploads/%'`, unique); err != nil {
			return nil, fmt.Errorf("query order upload references: %w", err)
		}
	}
	references := make([]string, 0, len(unique))
	for reference := range unique {
		references = append(references, reference)
	}
	return references, nil
}

func collectReferences(ctx context.Context, db *sql.DB, query string, unique map[string]struct{}) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var reference string
		if err := rows.Scan(&reference); err != nil {
			return err
		}
		if strings.TrimSpace(reference) != "" {
			unique[reference] = struct{}{}
		}
	}
	return rows.Err()
}

func (c *IntegrityChecker) referencePath(reference string) (string, bool) {
	const prefix = "/uploads/"
	if !strings.HasPrefix(reference, prefix) {
		return "", false
	}
	relative := filepath.FromSlash(strings.TrimPrefix(reference, prefix))
	path := filepath.Clean(filepath.Join(c.root, relative))
	rootPrefix := c.root + string(filepath.Separator)
	if path == c.root || !strings.HasPrefix(path, rootPrefix) {
		return "", false
	}
	return path, true
}

package assetstore

import (
	"context"
	"database/sql"
	"encoding/hex"
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
	CorruptFiles      int      `json:"corrupt_files"`
	InvalidReferences int      `json:"invalid_references"`
	MissingReferences []string `json:"missing_references,omitempty"`
	CorruptReferences []string `json:"corrupt_references,omitempty"`
	InvalidPaths      []string `json:"invalid_paths,omitempty"`
}

func (r IntegrityReport) Healthy() bool {
	return r.Configured && r.Writable && r.MissingFiles == 0 && r.CorruptFiles == 0 && r.InvalidReferences == 0
}

type IntegrityChecker struct {
	root   string
	source ReferenceSource
}

type ReferenceSource interface {
	References(context.Context) ([]string, error)
}

type SQLReferenceSource struct {
	productDB *sql.DB
	orderDB   *sql.DB
}

func NewIntegrityChecker(root string, productDB, orderDB *sql.DB) *IntegrityChecker {
	return NewIntegrityCheckerWithSource(root, &SQLReferenceSource{productDB: productDB, orderDB: orderDB})
}

func NewIntegrityCheckerWithSource(root string, source ReferenceSource) *IntegrityChecker {
	return &IntegrityChecker{root: filepath.Clean(root), source: source}
}

func (c *IntegrityChecker) Probe(_ context.Context) (IntegrityReport, error) {
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
	return report, nil
}

func (c *IntegrityChecker) Check(ctx context.Context) (IntegrityReport, error) {
	report, err := c.Probe(ctx)
	if err != nil {
		return report, err
	}
	if c.source == nil {
		return report, fmt.Errorf("upload reference source is not configured")
	}
	references, err := c.source.References(ctx)
	if err != nil {
		return report, err
	}
	report.ReferencedFiles = len(references)
	for _, reference := range references {
		path, ok := c.referencePath(reference)
		if !ok {
			report.InvalidPaths = append(report.InvalidPaths, reference)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if !os.IsNotExist(err) {
				return report, fmt.Errorf("stat upload reference %q: %w", reference, err)
			}
			report.MissingReferences = append(report.MissingReferences, reference)
			continue
		}
		if digest, ok := contentAddressFromPath(path); ok {
			valid, err := fileMatchesSHA256(path, digest)
			if err != nil {
				return report, fmt.Errorf("verify upload reference %q: %w", reference, err)
			}
			if !valid {
				report.CorruptReferences = append(report.CorruptReferences, reference)
			}
		}
	}
	report.MissingFiles = len(report.MissingReferences)
	report.CorruptFiles = len(report.CorruptReferences)
	report.InvalidReferences = len(report.InvalidPaths)
	return report, nil
}

func contentAddressFromPath(filename string) (string, bool) {
	name := filepath.Base(filename)
	extension := filepath.Ext(name)
	digest := strings.TrimSuffix(name, extension)
	if len(digest) != sha256HexLength {
		return "", false
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256HexLength/2 {
		return "", false
	}
	return digest, true
}

const sha256HexLength = 64

func (s *SQLReferenceSource) References(ctx context.Context) ([]string, error) {
	unique := make(map[string]struct{})
	if s.productDB != nil {
		if err := collectReferences(ctx, s.productDB,
			"SELECT image_url FROM product WHERE image_url LIKE '/uploads/%'", unique); err != nil {
			return nil, fmt.Errorf("query product upload references: %w", err)
		}
	}
	if s.orderDB != nil {
		if err := collectReferences(ctx, s.orderDB, `
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

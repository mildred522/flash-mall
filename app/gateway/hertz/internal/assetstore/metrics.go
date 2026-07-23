package assetstore

import "github.com/prometheus/client_golang/prometheus"

var (
	integrityReady = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flash_mall_upload_integrity_ready",
		Help: "Whether the upload store is configured, writable, and has a completed audit.",
	})
	integrityAuditErrors = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flash_mall_upload_integrity_audit_error",
		Help: "Whether the latest upload integrity audit failed.",
	})
	integrityMissingFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flash_mall_upload_integrity_missing_files",
		Help: "Number of referenced upload files missing in the latest audit.",
	})
	integrityCorruptFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flash_mall_upload_integrity_corrupt_files",
		Help: "Number of content-addressed upload files with invalid content in the latest audit.",
	})
	integrityInvalidReferences = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flash_mall_upload_integrity_invalid_references",
		Help: "Number of unsafe upload references found in the latest audit.",
	})
	integrityLastAuditTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flash_mall_upload_integrity_last_audit_timestamp_seconds",
		Help: "Unix timestamp of the latest completed upload integrity audit.",
	})
)

func init() {
	prometheus.MustRegister(
		integrityReady,
		integrityAuditErrors,
		integrityMissingFiles,
		integrityCorruptFiles,
		integrityInvalidReferences,
		integrityLastAuditTimestamp,
	)
}

func observeIntegritySnapshot(snapshot IntegritySnapshot) {
	if snapshot.Ready() {
		integrityReady.Set(1)
	} else {
		integrityReady.Set(0)
	}
	if snapshot.Error == "" {
		integrityAuditErrors.Set(0)
	} else {
		integrityAuditErrors.Set(1)
	}
	integrityMissingFiles.Set(float64(snapshot.Report.MissingFiles))
	integrityCorruptFiles.Set(float64(snapshot.Report.CorruptFiles))
	integrityInvalidReferences.Set(float64(snapshot.Report.InvalidReferences))
	integrityLastAuditTimestamp.Set(float64(snapshot.CheckedAt.Unix()))
}

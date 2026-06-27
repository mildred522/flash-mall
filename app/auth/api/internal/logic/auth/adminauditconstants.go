package auth

const (
	adminAuditResultSuccess = "success"
	adminAuditResultFail    = "fail"
)

const (
	adminAuditUserEnabled            = "admin_user_enabled"
	adminAuditUserDisabled           = "admin_user_disabled"
	adminAuditUserStatusUpdateFailed = "admin_user_status_update_failed"
)

const (
	adminAuditReasonInvalidUserID = "invalid_user_id"
	adminAuditReasonInvalidStatus = "invalid_status"
	adminAuditReasonNotFound      = "not_found"
	adminAuditReasonStoreFailed   = "store_failed"

	AdminAuditReasonSelfDisableBlocked = "self_disable_blocked"
)

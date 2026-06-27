package auth

const (
	auditResultSuccess = "success"
	auditResultFail    = "fail"
	auditResultBlocked = "blocked"
)

const (
	auditEventLoginCodeFail        = "login_code_fail"
	auditEventLoginCodeSuccess     = "login_code_success"
	auditEventLoginPasswordFail    = "login_password_fail"
	auditEventLoginPasswordSuccess = "login_password_success"
	auditEventLogoutAllSuccess     = "logout_all_success"
	auditEventLogoutSuccess        = "logout_success"
	auditEventRefreshSuccess       = "refresh_success"
	auditEventRegisterFail         = "register_fail"
	auditEventRegisterSuccess      = "register_success"
	auditEventResetPasswordFail    = "reset_password_fail"
	auditEventResetPasswordSuccess = "reset_password_success"
	auditEventSendCodeBlocked      = "send_code_blocked"
	auditEventSendCodeSuccess      = "send_code_success"
)

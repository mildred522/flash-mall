package types

type LoginReq struct {
	UserId     int64  `json:"user_id,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	Phone      string `json:"phone,optional"`   //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	Password   string `json:"password"`
	DeviceType string `json:"device_type,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	ClientIP   string `json:"-"`
	UserAgent  string `json:"-"`
}

type LoginResp struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresAt    int64  `json:"expires_at"`
	UserId       int64  `json:"user_id"`
	DisplayName  string `json:"display_name"`
	Phone        string `json:"phone"`
	RefreshToken string `json:"-"`
}

type SendCodeReq struct {
	Phone     string `json:"phone"`
	Scene     string `json:"scene"`
	ClientIP  string `json:"-"`
	UserAgent string `json:"-"`
}

type SendCodeResp struct {
	Sent      bool   `json:"sent"`
	ExpiresAt int64  `json:"expires_at"`
	DebugCode string `json:"debug_code,omitempty"`
}

type RegisterReq struct {
	Phone       string `json:"phone"`
	Password    string `json:"password"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	DeviceType  string `json:"device_type,optional"`  //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	ClientIP    string `json:"-"`
	UserAgent   string `json:"-"`
}

type LoginCodeReq struct {
	Phone      string `json:"phone"`
	Code       string `json:"code"`
	DeviceType string `json:"device_type,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	ClientIP   string `json:"-"`
	UserAgent  string `json:"-"`
}

type LogoutResp struct {
	Success bool `json:"success"`
}

type ForgotPasswordReq struct {
	Phone string `json:"phone"`
}

type ResetPasswordReq struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
	ClientIP    string `json:"-"`
	UserAgent   string `json:"-"`
}

type MeResp struct {
	UserId      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
	Role        string `json:"role"`
}

type SecurityEventItem struct {
	EventType string `json:"event_type"`
	Result    string `json:"result"`
	UserId    int64  `json:"user_id"`
	Subject   string `json:"subject"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	CreatedAt int64  `json:"created_at"`
}

type SecurityEventsRecentReq struct {
	Limit     int64  `form:"limit,optional,default=20"`
	UserId    int64  `form:"user_id,optional"`
	EventType string `form:"event_type,optional"`
	Result    string `form:"result,optional"`
	Keyword   string `form:"keyword,optional"`
}

type AdminSecurityEventRecordReq struct {
	EventType string `json:"event_type"`
	Result    string `json:"result,optional"`  //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
	Subject   string `json:"subject,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
}

type SecurityEventsRecentResp struct {
	Items []SecurityEventItem `json:"items"`
}

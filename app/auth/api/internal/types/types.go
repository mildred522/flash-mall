package types

type LoginReq struct {
	UserId     int64  `json:"user_id,optional"`
	Phone      string `json:"phone,optional"`
	Password   string `json:"password"`
	DeviceType string `json:"device_type,optional"`
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
	DisplayName string `json:"display_name,optional"`
	DeviceType  string `json:"device_type,optional"`
}

type LoginCodeReq struct {
	Phone      string `json:"phone"`
	Code       string `json:"code"`
	DeviceType string `json:"device_type,optional"`
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
	CreatedAt int64  `json:"created_at"`
}

type SecurityEventsRecentResp struct {
	Items []SecurityEventItem `json:"items"`
}

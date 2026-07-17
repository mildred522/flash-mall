package handler

type MerchantMeItem struct {
	MerchantID int64  `json:"merchant_id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     int64  `json:"status"`
}

type MerchantMeResp struct {
	Items []MerchantMeItem `json:"items"`
}

type MerchantStoreProfile struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	LogoURL      string `json:"logo_url"`
	BannerURL    string `json:"banner_url"`
	Description  string `json:"description"`
	Version      int64  `json:"version"`
}

type merchantStoreUpdateReq struct {
	LogoURL         string `json:"logo_url"`
	BannerURL       string `json:"banner_url"`
	Description     string `json:"description"`
	ExpectedVersion int64  `json:"expected_version"`
}

type MerchantApplyReq struct {
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone,omitempty"`
}

type MerchantApplyResp struct {
	ApplyID int64  `json:"apply_id"`
	Status  string `json:"status"`
}

type MerchantApplicationItem struct {
	ApplyID      int64  `json:"apply_id"`
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone"`
	Status       int64  `json:"status"`
	StatusText   string `json:"status_text"`
	MerchantID   int64  `json:"merchant_id"`
	AuditRemark  string `json:"audit_remark"`
	CreateTime   string `json:"create_time"`
	AuditTime    string `json:"audit_time"`
}

type MerchantApplicationResp struct {
	Application *MerchantApplicationItem `json:"application"`
}

type AdminMerchantApplicationListReq struct {
	Status   int64 `json:"status"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
}

type AdminMerchantApplicationItem struct {
	MerchantApplicationItem
	UserID     int64 `json:"user_id"`
	OperatorID int64 `json:"operator_id"`
}

type AdminMerchantApplicationListResp struct {
	Items    []AdminMerchantApplicationItem `json:"items"`
	Total    int64                          `json:"total"`
	Page     int64                          `json:"page"`
	PageSize int64                          `json:"page_size"`
}

type MerchantDashboardStatsResp struct {
	MerchantID       int64 `json:"merchant_id"`
	OrderCount       int64 `json:"order_count"`
	PaidOrderCount   int64 `json:"paid_order_count"`
	ShipPendingCount int64 `json:"ship_pending_count"`
	RefundPending    int64 `json:"refund_pending_count"`
	SalesAmountFen   int64 `json:"sales_amount_fen"`
}

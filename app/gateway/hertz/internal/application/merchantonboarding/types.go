package merchantonboarding

import "context"

const (
	StatusPending  int64 = 0
	StatusApproved int64 = 1
	StatusRejected int64 = 2
)

const (
	ReasonInvalidArgument = "invalid_argument"
	ReasonAlreadyActive   = "already_active"
	ReasonNotFound        = "not_found"
	ReasonAlreadyAudited  = "already_audited"
)

const (
	RejectAlreadyActive  = "already_active"
	RejectNotFound       = "not_found"
	RejectAlreadyAudited = "already_audited"
)

type SubmitInput struct {
	UserID       int64  `json:"-"`
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone,omitempty"`
}

type SubmitResult struct {
	ApplyID   int64
	Status    int64
	Rejection string
}

type ListQuery struct {
	Status   int64
	Page     int64
	PageSize int64
}

type Application struct {
	ApplyID      int64  `json:"apply_id"`
	UserID       int64  `json:"user_id"`
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone"`
	Status       int64  `json:"status"`
	StatusText   string `json:"status_text"`
	MerchantID   int64  `json:"merchant_id"`
	AuditRemark  string `json:"audit_remark"`
	OperatorID   int64  `json:"operator_id"`
	CreateTime   string `json:"create_time"`
	AuditTime    string `json:"audit_time"`
}

type ListResult struct {
	Items    []Application `json:"items"`
	Total    int64         `json:"total"`
	Page     int64         `json:"page"`
	PageSize int64         `json:"page_size"`
}

type AuditInput struct {
	ApplyID    int64  `json:"apply_id"`
	Approve    bool   `json:"approve"`
	Remark     string `json:"remark,omitempty"`
	OperatorID int64  `json:"-"`
}

type AuditResult struct {
	ApplyID    int64  `json:"apply_id"`
	MerchantID int64  `json:"merchant_id"`
	Status     int64  `json:"status"`
	Idempotent bool   `json:"idempotent,omitempty"`
	Rejection  string `json:"-"`
}

type Repository interface {
	Submit(context.Context, SubmitInput) (SubmitResult, error)
	List(context.Context, ListQuery) (ListResult, error)
	Audit(context.Context, AuditInput) (AuditResult, error)
}

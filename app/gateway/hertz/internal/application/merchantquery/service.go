package merchantquery

import (
	"context"

	"flash-mall/app/common/apperror"
)

type Merchant struct {
	MerchantID int64  `json:"merchant_id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     int64  `json:"status"`
}

type MeResponse struct {
	Items []Merchant `json:"items"`
}

type Application struct {
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

type ApplicationResponse struct {
	Application *Application `json:"application"`
}

type DashboardStats struct {
	MerchantID       int64 `json:"merchant_id"`
	OrderCount       int64 `json:"order_count"`
	PaidOrderCount   int64 `json:"paid_order_count"`
	ShipPendingCount int64 `json:"ship_pending_count"`
	RefundPending    int64 `json:"refund_pending_count"`
	SalesAmountFen   int64 `json:"sales_amount_fen"`
}

type Repository interface {
	MerchantsByUser(context.Context, int64) ([]Merchant, error)
	LatestApplication(context.Context, int64) (Application, bool, error)
	Dashboard(context.Context, int64) (DashboardStats, error)
	FirstMerchantByUser(context.Context, int64) (int64, bool, error)
	UserCanAccessMerchant(context.Context, int64, int64) (bool, error)
	DefaultActiveMerchant(context.Context) (int64, bool, error)
}

type ScopeRequest struct {
	UserID     int64
	MerchantID int64
	Admin      bool
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Me(ctx context.Context, userID int64) (MeResponse, error) {
	items, err := s.repository.MerchantsByUser(ctx, userID)
	if items == nil {
		items = []Merchant{}
	}
	return MeResponse{Items: items}, err
}

func (s *Service) LatestApplication(ctx context.Context, userID int64) (ApplicationResponse, error) {
	application, found, err := s.repository.LatestApplication(ctx, userID)
	if err != nil || !found {
		return ApplicationResponse{}, err
	}
	application.StatusText = applicationStatusText(application.Status)
	return ApplicationResponse{Application: &application}, nil
}

func (s *Service) Dashboard(ctx context.Context, merchantID int64) (DashboardStats, error) {
	return s.repository.Dashboard(ctx, merchantID)
}

func (s *Service) ResolveScope(ctx context.Context, request ScopeRequest) (int64, error) {
	if request.UserID <= 0 {
		return 0, apperror.New(apperror.CodeUnauthorized, "merchant login required")
	}
	if request.Admin {
		if request.MerchantID > 0 {
			return request.MerchantID, nil
		}
		merchantID, found, err := s.repository.DefaultActiveMerchant(ctx)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, apperror.New(apperror.CodeMerchantNotFound, "active merchant not found")
		}
		return merchantID, nil
	}
	if request.MerchantID > 0 {
		allowed, err := s.repository.UserCanAccessMerchant(ctx, request.UserID, request.MerchantID)
		if err != nil {
			return 0, err
		}
		if !allowed {
			return 0, apperror.New(apperror.CodeForbidden, "merchant access denied")
		}
		return request.MerchantID, nil
	}
	merchantID, found, err := s.repository.FirstMerchantByUser(ctx, request.UserID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, apperror.New(apperror.CodeMerchantNotBound, "merchant access required")
	}
	return merchantID, nil
}

func applicationStatusText(status int64) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "approved"
	case 2:
		return "rejected"
	default:
		return "unknown"
	}
}

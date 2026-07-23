package stockaudit

import (
	"context"
	"strings"
)

type Query struct {
	Page       int64
	PageSize   int64
	ProductID  int64
	MerchantID int64
	OrderID    string
	ChangeType string
}

type Item struct {
	ID                 int64  `json:"id"`
	ProductID          int64  `json:"product_id"`
	OrderID            string `json:"order_id"`
	ChangeType         string `json:"change_type"`
	Delta              int64  `json:"delta"`
	BeforeAvailable    int64  `json:"before_available"`
	AfterAvailable     int64  `json:"after_available"`
	Reason             string `json:"reason"`
	RequestID          string `json:"request_id"`
	TraceID            string `json:"trace_id"`
	OperatorUserID     int64  `json:"operator_user_id"`
	OperatorMerchantID int64  `json:"operator_merchant_id"`
	OperatorRole       string `json:"operator_role"`
	CreateTime         string `json:"create_time"`
}

type List struct {
	Items    []Item `json:"items"`
	Total    int64  `json:"total"`
	Page     int64  `json:"page"`
	PageSize int64  `json:"page_size"`
}

type Repository interface {
	List(context.Context, Query) ([]Item, int64, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, query Query) (List, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 20
	}
	query.OrderID = strings.TrimSpace(query.OrderID)
	query.ChangeType = strings.ToUpper(strings.TrimSpace(query.ChangeType))
	items, total, err := s.repository.List(ctx, query)
	if items == nil {
		items = []Item{}
	}
	return List{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, err
}

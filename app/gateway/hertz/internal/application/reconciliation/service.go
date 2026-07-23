package reconciliation

import (
	"context"
	"strings"
)

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, query Query) (List, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.IssueType = strings.TrimSpace(query.IssueType)
	query.OrderID = strings.TrimSpace(query.OrderID)
	result, err := s.repository.List(ctx, query)
	if result.Items == nil {
		result.Items = []Issue{}
	}
	return result, err
}

func (s *Service) Scan(ctx context.Context) (int64, error) {
	return s.repository.Scan(ctx)
}

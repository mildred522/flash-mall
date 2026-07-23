package showcase

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CandidateQuery struct {
	Keyword    string
	MerchantID int64
}

type CandidateFeature struct {
	ProductID            int64
	MerchantID           int64
	Sales7d              int64
	StockAvailable       int64
	HasPromotion         bool
	CreatedAt            time.Time
	AgeDays              int
	CurrentMerchantSlots int
}

type CandidateRepository interface {
	CandidateFeatures(context.Context, CandidateQuery) ([]CandidateFeature, error)
}

func (s *Service) CandidateFeatures(ctx context.Context, query CandidateQuery) ([]CandidateFeature, error) {
	repository, ok := s.repository.(CandidateRepository)
	if !ok {
		return nil, fmt.Errorf("showcase candidate repository is not configured")
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	return repository.CandidateFeatures(ctx, query)
}

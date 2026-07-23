package campaign

import (
	"context"
	"errors"
	"strings"
)

const ReasonInvalidArgument = "invalid_argument"

type Fault struct {
	Reason  string
	Message string
}

func (f *Fault) Error() string { return f.Message }

func AsFault(err error) (*Fault, bool) {
	var fault *Fault
	return fault, errors.As(err, &fault)
}

type Item struct {
	CampaignID    int64  `json:"campaign_id"`
	ProductID     int64  `json:"product_id"`
	ProductName   string `json:"product_name"`
	Name          string `json:"name"`
	CampaignStock int64  `json:"campaign_stock"`
	PerUserLimit  int64  `json:"per_user_limit"`
	StartsAt      string `json:"starts_at"`
	EndsAt        string `json:"ends_at"`
	Status        int64  `json:"status"`
}

type UpsertInput struct {
	CampaignID    int64  `json:"campaign_id"`
	ProductID     int64  `json:"product_id"`
	CampaignStock int64  `json:"campaign_stock"`
	PerUserLimit  int64  `json:"per_user_limit"`
	Status        int64  `json:"status"`
	Name          string `json:"name"`
	StartsAt      string `json:"starts_at"`
	EndsAt        string `json:"ends_at"`
}

type Repository interface {
	List(context.Context) ([]Item, error)
	Upsert(context.Context, UpsertInput) (int64, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context) ([]Item, error) {
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Item{}
	}
	return items, nil
}

func (s *Service) Upsert(ctx context.Context, input UpsertInput) (int64, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.StartsAt = strings.TrimSpace(input.StartsAt)
	input.EndsAt = strings.TrimSpace(input.EndsAt)
	if input.ProductID <= 0 || input.Name == "" || input.CampaignStock < 0 {
		return 0, &Fault{Reason: ReasonInvalidArgument, Message: "product_id, name and campaign_stock are required"}
	}
	if input.PerUserLimit <= 0 {
		input.PerUserLimit = 1
	}
	return s.repository.Upsert(ctx, input)
}

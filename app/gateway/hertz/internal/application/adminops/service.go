package adminops

import (
	"context"
	"errors"
	"strings"
)

type Fault struct {
	Reason  string
	Message string
}

func (f *Fault) Error() string { return f.Message }

func AsFault(err error) (*Fault, bool) {
	var fault *Fault
	return fault, errors.As(err, &fault)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Dashboard(ctx context.Context) (DashboardStats, error) {
	return s.repository.Dashboard(ctx)
}

func (s *Service) Events(ctx context.Context, query EventQuery) (EventList, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.EventType = strings.TrimSpace(query.EventType)
	query.AggregateID = strings.TrimSpace(query.AggregateID)
	result, err := s.repository.Events(ctx, query)
	if result.Items == nil {
		result.Items = []Event{}
	}
	return result, err
}

func (s *Service) RetryEvent(ctx context.Context, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return &Fault{Reason: ReasonInvalidArgument, Message: "event_id is required"}
	}
	return s.repository.RetryEvent(ctx, eventID)
}

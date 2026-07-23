package useraddress

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

func (s *Service) List(ctx context.Context, userID int64) (ListResult, error) {
	if userID <= 0 {
		return ListResult{}, fault(ReasonInvalidArgument, "user_id is required")
	}
	items, err := s.repository.List(ctx, userID)
	if items == nil {
		items = []Address{}
	}
	return ListResult{Items: items}, err
}

func (s *Service) Upsert(ctx context.Context, input UpsertInput) (UpsertResult, error) {
	input.ReceiverName = strings.TrimSpace(input.ReceiverName)
	input.ReceiverPhone = strings.TrimSpace(input.ReceiverPhone)
	input.Province = strings.TrimSpace(input.Province)
	input.City = strings.TrimSpace(input.City)
	input.District = strings.TrimSpace(input.District)
	input.Detail = strings.TrimSpace(input.Detail)
	if input.UserID <= 0 || input.AddressID < 0 ||
		input.ReceiverName == "" || input.ReceiverPhone == "" || input.Detail == "" {
		return UpsertResult{}, fault(ReasonInvalidArgument,
			"user_id, receiver_name, receiver_phone and detail are required")
	}
	record, err := s.repository.Upsert(ctx, input)
	if err != nil {
		return UpsertResult{}, err
	}
	if !record.Found {
		return UpsertResult{}, fault(ReasonNotFound, "user address not found")
	}
	return UpsertResult{AddressID: record.AddressID}, nil
}

func fault(reason, message string) error { return &Fault{Reason: reason, Message: message} }

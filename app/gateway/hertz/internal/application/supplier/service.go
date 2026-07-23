package supplier

import (
	"context"
	"errors"
	"strings"
)

const (
	StatusActive   int64 = 1
	StatusInactive int64 = 2
)

const (
	ReasonInvalidArgument   = "invalid_argument"
	ReasonSupplierNotFound  = "supplier_not_found"
	ReasonHasActiveProducts = "has_active_products"
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

type ListQuery struct {
	Page     int64
	PageSize int64
	Status   int64
	Keyword  string
}

type Record struct {
	SupplierID     int64  `json:"supplier_id"`
	Name           string `json:"name"`
	Status         int64  `json:"status"`
	StatusText     string `json:"status_text"`
	ProductCount   int64  `json:"product_count"`
	ActiveProducts int64  `json:"active_products"`
}

type CreateInput struct {
	Name   string `json:"name"`
	Status int64  `json:"status,omitempty"`
}

type UpdateInput struct {
	SupplierID int64  `json:"supplier_id"`
	Name       string `json:"name,omitempty"`
	Status     *int64 `json:"status,omitempty"`
}

type Changes struct {
	Name   *string
	Status *int64
}

type UpdateResult struct {
	Found             bool
	HasActiveProducts bool
}

type MutationResult struct{ SupplierID int64 }

type Repository interface {
	List(context.Context, ListQuery) ([]Record, int64, error)
	Find(context.Context, int64) (Record, bool, error)
	Insert(context.Context, Record) (int64, error)
	Update(context.Context, int64, Changes) (UpdateResult, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, query ListQuery) ([]Record, int64, error) {
	records, total, err := s.repository.List(ctx, query)
	for index := range records {
		records[index].StatusText = statusText(records[index].Status)
	}
	return records, total, err
}

func (s *Service) Detail(ctx context.Context, supplierID int64) (Record, error) {
	record, found, err := s.repository.Find(ctx, supplierID)
	if err != nil {
		return Record{}, err
	}
	if !found {
		return Record{}, fault(ReasonSupplierNotFound, "supplier not found")
	}
	record.StatusText = statusText(record.Status)
	return record, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (MutationResult, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return MutationResult{}, fault(ReasonInvalidArgument, "name required")
	}
	if input.Status == 0 {
		input.Status = StatusActive
	}
	if !validStatus(input.Status) {
		return MutationResult{}, fault(ReasonInvalidArgument, "status must be 1 or 2")
	}
	id, err := s.repository.Insert(ctx, Record{Name: input.Name, Status: input.Status})
	return MutationResult{SupplierID: id}, err
}

func (s *Service) Update(ctx context.Context, input UpdateInput) error {
	if input.SupplierID <= 0 {
		return fault(ReasonInvalidArgument, "supplier_id required")
	}
	changes := Changes{Status: input.Status}
	if name := strings.TrimSpace(input.Name); name != "" {
		changes.Name = &name
	}
	if changes.Name == nil && changes.Status == nil {
		return fault(ReasonInvalidArgument, "no fields to update")
	}
	if changes.Status != nil && !validStatus(*changes.Status) {
		return fault(ReasonInvalidArgument, "status must be 1 or 2")
	}
	result, err := s.repository.Update(ctx, input.SupplierID, changes)
	if err != nil {
		return err
	}
	if !result.Found {
		return fault(ReasonSupplierNotFound, "supplier not found")
	}
	if result.HasActiveProducts {
		return fault(ReasonHasActiveProducts, "supplier has active products")
	}
	return nil
}

func validStatus(status int64) bool { return status == StatusActive || status == StatusInactive }

func statusText(status int64) string {
	if status == StatusActive {
		return "active"
	}
	if status == StatusInactive {
		return "inactive"
	}
	return "unknown"
}

func fault(reason, message string) error { return &Fault{Reason: reason, Message: message} }

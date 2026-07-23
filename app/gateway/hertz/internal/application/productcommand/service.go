package productcommand

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

func (s *Service) Create(ctx context.Context, input CreateInput) (MutationResult, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.Name == "" || input.MerchantID <= 0 || input.SupplierID <= 0 ||
		input.OriginPriceFen < 0 || input.SalePriceFen < 0 || input.StockAvailable < 0 {
		return MutationResult{}, fault(ReasonInvalidArgument,
			"name, merchant_id, non-negative prices, stock and supplier_id are required")
	}
	if input.SalePriceFen > input.OriginPriceFen {
		return MutationResult{}, fault(ReasonInvalidPrice, "sale_price_fen must be <= origin_price_fen")
	}
	if input.Status == 0 {
		input.Status = StatusActive
	}
	if !validStatus(input.Status) {
		return MutationResult{}, fault(ReasonInvalidArgument, "status must be 1 or 2")
	}
	result, err := s.repository.Create(ctx, input)
	if err != nil {
		return MutationResult{}, err
	}
	if err := rejectionFault(result.Rejection); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{ProductID: result.ProductID}, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.ProductID <= 0 || input.MerchantID < 0 {
		return fault(ReasonInvalidArgument, "product_id required")
	}
	if input.SalePriceFen != nil && *input.SalePriceFen < 0 ||
		input.OriginPriceFen != nil && *input.OriginPriceFen < 0 {
		return fault(ReasonInvalidArgument, "prices must be non-negative")
	}
	if input.SupplierID != nil && *input.SupplierID <= 0 {
		return fault(ReasonInvalidArgument, "supplier_id required")
	}
	if input.Status != nil && !validStatus(*input.Status) {
		return fault(ReasonInvalidArgument, "status must be 1 or 2")
	}
	if input.Name == "" && input.ImageURL == "" && input.SalePriceFen == nil && input.OriginPriceFen == nil &&
		input.SupplierID == nil && input.Status == nil {
		return fault(ReasonInvalidArgument, "no fields to update")
	}
	result, err := s.repository.Update(ctx, input)
	if err != nil {
		return err
	}
	if !result.Found {
		return fault(ReasonProductNotFound, "product not found")
	}
	return rejectionFault(result.Rejection)
}

func validStatus(status int64) bool { return status == StatusActive || status == StatusInactive }

func rejectionFault(rejection string) error {
	switch rejection {
	case "":
		return nil
	case RejectInvalidPrice:
		return fault(ReasonInvalidPrice, "sale_price_fen must be <= origin_price_fen")
	case RejectSupplierNotFound:
		return fault(ReasonSupplierNotFound, "active supplier not found")
	case RejectMerchantNotFound:
		return fault(ReasonMerchantNotFound, "active merchant not found")
	default:
		return errors.New("unknown product mutation rejection: " + rejection)
	}
}

func fault(reason, message string) error { return &Fault{Reason: reason, Message: message} }

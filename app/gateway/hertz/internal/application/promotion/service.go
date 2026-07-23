package promotion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TypeLimitedPrice = "LIMITED_PRICE"
	StatusActive     = 1
	StatusInactive   = 2
)

const (
	ReasonInvalidArgument   = "invalid_argument"
	ReasonProductNotFound   = "product_not_found"
	ReasonPromotionNotFound = "promotion_not_found"
	ReasonInvalidDiscount   = "invalid_discount"
	ReasonWindowConflict    = "window_conflict"
	ReasonInvalidWindow     = "invalid_window"
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
	Page         int64
	PageSize     int64
	ProductID    int64
	Status       int64
	EffectStatus string
	Keyword      string
}

type Item struct {
	PromotionID      int64  `json:"promotion_id"`
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	OriginPriceFen   int64  `json:"origin_price_fen"`
	SalePriceFen     int64  `json:"sale_price_fen"`
	Type             string `json:"type"`
	DiscountValue    int64  `json:"discount_value"`
	ThresholdAmount  int64  `json:"threshold_amount"`
	StartsAt         string `json:"starts_at"`
	EndsAt           string `json:"ends_at"`
	EffectStatus     string `json:"effect_status"`
	EffectStatusText string `json:"effect_status_text"`
	Status           int64  `json:"status"`
	StatusText       string `json:"status_text"`
}

type Record struct {
	ID              int64
	ProductID       int64
	ProductName     string
	OriginPriceFen  int64
	SalePriceFen    int64
	Type            string
	DiscountValue   int64
	ThresholdAmount int64
	StartsAt        *time.Time
	EndsAt          *time.Time
	Status          int64
}

type CreateInput struct {
	ProductID       int64  `json:"product_id"`
	Type            string `json:"type,omitempty"`
	DiscountValue   int64  `json:"discount_value"`
	ThresholdAmount int64  `json:"threshold_amount,omitempty"`
	StartsAt        string `json:"starts_at,omitempty"`
	EndsAt          string `json:"ends_at,omitempty"`
	Status          int64  `json:"status,omitempty"`
}

type UpdateInput struct {
	PromotionID     int64   `json:"promotion_id"`
	ProductID       *int64  `json:"product_id,omitempty"`
	DiscountValue   *int64  `json:"discount_value,omitempty"`
	ThresholdAmount *int64  `json:"threshold_amount,omitempty"`
	StartsAt        *string `json:"starts_at,omitempty"`
	EndsAt          *string `json:"ends_at,omitempty"`
	Status          *int64  `json:"status,omitempty"`
}

type OptionalTime struct {
	Set   bool
	Value *time.Time
}

type Changes struct {
	ProductID       *int64
	DiscountValue   *int64
	ThresholdAmount *int64
	StartsAt        OptionalTime
	EndsAt          OptionalTime
	Status          *int64
}

type MutationResult struct {
	PromotionID        int64
	AffectedProductIDs []int64
}

type Repository interface {
	List(context.Context, ListQuery) ([]Record, int64, error)
	Find(context.Context, int64) (Record, bool, error)
	ProductSalePrice(context.Context, int64) (int64, bool, error)
	HasActiveConflict(context.Context, int64, int64, *time.Time, *time.Time) (bool, error)
	Insert(context.Context, Record) (int64, error)
	Update(context.Context, int64, Changes) (bool, error)
	WindowAffectedProductIDs(context.Context, time.Time, int64, int64) ([]int64, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, now: now}
}

func (s *Service) List(ctx context.Context, query ListQuery) ([]Item, int64, error) {
	records, total, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	items := make([]Item, 0, len(records))
	for _, record := range records {
		items = append(items, present(record, s.now()))
	}
	return items, total, nil
}

func (s *Service) Detail(ctx context.Context, promotionID int64) (Item, error) {
	record, found, err := s.repository.Find(ctx, promotionID)
	if err != nil {
		return Item{}, err
	}
	if !found {
		return Item{}, fault(ReasonPromotionNotFound, "promotion not found")
	}
	return present(record, s.now()), nil
}

func (s *Service) WindowAffectedProductIDs(ctx context.Context, windowMinutes, limit int64) ([]int64, error) {
	if windowMinutes <= 0 || windowMinutes > 24*60 {
		windowMinutes = 120
	}
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	return s.repository.WindowAffectedProductIDs(ctx, s.now(), windowMinutes, limit)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (MutationResult, error) {
	input.Type = normalizeType(input.Type)
	if input.ProductID <= 0 || input.DiscountValue <= 0 || input.ThresholdAmount < 0 {
		return MutationResult{}, fault(ReasonInvalidArgument, "product_id, positive discount_value and non-negative threshold_amount are required")
	}
	if input.Type != TypeLimitedPrice {
		return MutationResult{}, fault(ReasonInvalidArgument, "unsupported promotion type")
	}
	if input.Status == 0 {
		input.Status = StatusActive
	}
	if !validStatus(input.Status) {
		return MutationResult{}, fault(ReasonInvalidArgument, "status must be 1 or 2")
	}
	startsAt, err := parseTime(input.StartsAt)
	if err != nil {
		return MutationResult{}, fault(ReasonInvalidArgument, err.Error())
	}
	endsAt, err := parseTime(input.EndsAt)
	if err != nil {
		return MutationResult{}, fault(ReasonInvalidArgument, err.Error())
	}
	if !validWindow(startsAt, endsAt) {
		return MutationResult{}, fault(ReasonInvalidWindow, "ends_at must be after starts_at")
	}
	salePrice, found, err := s.repository.ProductSalePrice(ctx, input.ProductID)
	if err != nil {
		return MutationResult{}, err
	}
	if !found {
		return MutationResult{}, fault(ReasonProductNotFound, "product not found")
	}
	if input.DiscountValue > salePrice {
		return MutationResult{}, fault(ReasonInvalidDiscount, "discount_value must be <= product sale_price_fen")
	}
	if input.Status == StatusActive {
		conflict, err := s.repository.HasActiveConflict(ctx, input.ProductID, 0, startsAt, endsAt)
		if err != nil {
			return MutationResult{}, err
		}
		if conflict {
			return MutationResult{}, fault(ReasonWindowConflict, "active limited price promotion window overlaps")
		}
	}
	id, err := s.repository.Insert(ctx, Record{
		ProductID: input.ProductID, Type: input.Type, DiscountValue: input.DiscountValue,
		ThresholdAmount: input.ThresholdAmount, StartsAt: startsAt, EndsAt: endsAt, Status: input.Status,
	})
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{PromotionID: id, AffectedProductIDs: []int64{input.ProductID}}, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (MutationResult, error) {
	if input.PromotionID <= 0 {
		return MutationResult{}, fault(ReasonInvalidArgument, "promotion_id required")
	}
	if input.ProductID == nil && input.DiscountValue == nil && input.ThresholdAmount == nil && input.StartsAt == nil && input.EndsAt == nil && input.Status == nil {
		return MutationResult{}, fault(ReasonInvalidArgument, "no fields to update")
	}
	current, found, err := s.repository.Find(ctx, input.PromotionID)
	if err != nil {
		return MutationResult{}, err
	}
	if !found {
		return MutationResult{}, fault(ReasonPromotionNotFound, "promotion not found")
	}

	changes := Changes{ProductID: input.ProductID, DiscountValue: input.DiscountValue, ThresholdAmount: input.ThresholdAmount, Status: input.Status}
	finalProductID, finalDiscount, finalStatus := current.ProductID, current.DiscountValue, current.Status
	finalStartsAt, finalEndsAt := current.StartsAt, current.EndsAt
	if input.ProductID != nil {
		if *input.ProductID <= 0 {
			return MutationResult{}, fault(ReasonInvalidArgument, "product_id required")
		}
		finalProductID = *input.ProductID
	}
	if input.DiscountValue != nil {
		if *input.DiscountValue <= 0 {
			return MutationResult{}, fault(ReasonInvalidArgument, "discount_value must be positive")
		}
		finalDiscount = *input.DiscountValue
	}
	if input.ThresholdAmount != nil && *input.ThresholdAmount < 0 {
		return MutationResult{}, fault(ReasonInvalidArgument, "threshold_amount must be non-negative")
	}
	if input.Status != nil {
		if !validStatus(*input.Status) {
			return MutationResult{}, fault(ReasonInvalidArgument, "status must be 1 or 2")
		}
		finalStatus = *input.Status
	}
	if input.StartsAt != nil {
		value, parseErr := parseTime(*input.StartsAt)
		if parseErr != nil {
			return MutationResult{}, fault(ReasonInvalidArgument, parseErr.Error())
		}
		changes.StartsAt = OptionalTime{Set: true, Value: value}
		finalStartsAt = value
	}
	if input.EndsAt != nil {
		value, parseErr := parseTime(*input.EndsAt)
		if parseErr != nil {
			return MutationResult{}, fault(ReasonInvalidArgument, parseErr.Error())
		}
		changes.EndsAt = OptionalTime{Set: true, Value: value}
		finalEndsAt = value
	}
	if !validWindow(finalStartsAt, finalEndsAt) {
		return MutationResult{}, fault(ReasonInvalidWindow, "ends_at must be after starts_at")
	}
	if input.ProductID != nil || input.DiscountValue != nil {
		salePrice, productFound, priceErr := s.repository.ProductSalePrice(ctx, finalProductID)
		if priceErr != nil {
			return MutationResult{}, priceErr
		}
		if !productFound {
			return MutationResult{}, fault(ReasonProductNotFound, "product not found")
		}
		if finalDiscount > salePrice {
			return MutationResult{}, fault(ReasonInvalidDiscount, "discount_value must be <= product sale_price_fen")
		}
	}
	if finalStatus == StatusActive && (input.ProductID != nil || input.Status != nil || input.StartsAt != nil || input.EndsAt != nil) {
		conflict, conflictErr := s.repository.HasActiveConflict(ctx, finalProductID, input.PromotionID, finalStartsAt, finalEndsAt)
		if conflictErr != nil {
			return MutationResult{}, conflictErr
		}
		if conflict {
			return MutationResult{}, fault(ReasonWindowConflict, "active limited price promotion window overlaps")
		}
	}
	updated, err := s.repository.Update(ctx, input.PromotionID, changes)
	if err != nil {
		return MutationResult{}, err
	}
	if !updated {
		return MutationResult{}, fault(ReasonPromotionNotFound, "promotion not found")
	}
	return MutationResult{PromotionID: input.PromotionID, AffectedProductIDs: uniqueIDs(current.ProductID, finalProductID)}, nil
}

func normalizeType(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return TypeLimitedPrice
	}
	return value
}

func parseTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time: %s", value)
}

func validStatus(status int64) bool { return status == StatusActive || status == StatusInactive }

func validWindow(startsAt, endsAt *time.Time) bool {
	return startsAt == nil || endsAt == nil || !endsAt.Before(*startsAt)
}

func present(record Record, now time.Time) Item {
	effectStatus, effectStatusText := "active", "生效中"
	if record.Status != StatusActive {
		effectStatus, effectStatusText = "inactive", "停用"
	} else if record.StartsAt != nil && record.StartsAt.After(now) {
		effectStatus, effectStatusText = "scheduled", "未开始"
	} else if record.EndsAt != nil && record.EndsAt.Before(now) {
		effectStatus, effectStatusText = "expired", "已结束"
	}
	statusText := "unknown"
	if record.Status == StatusActive {
		statusText = "active"
	} else if record.Status == StatusInactive {
		statusText = "inactive"
	}
	return Item{
		PromotionID: record.ID, ProductID: record.ProductID, ProductName: record.ProductName,
		OriginPriceFen: record.OriginPriceFen, SalePriceFen: record.SalePriceFen, Type: record.Type,
		DiscountValue: record.DiscountValue, ThresholdAmount: record.ThresholdAmount,
		StartsAt: formatTime(record.StartsAt), EndsAt: formatTime(record.EndsAt),
		EffectStatus: effectStatus, EffectStatusText: effectStatusText, Status: record.Status, StatusText: statusText,
	}
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func uniqueIDs(values ...int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fault(reason, message string) *Fault { return &Fault{Reason: reason, Message: message} }

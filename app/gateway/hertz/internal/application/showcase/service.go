package showcase

import (
	"context"
	"errors"
	"fmt"
)

const CurrentID int64 = 1

var (
	ErrInvalidDraft    = errors.New("invalid showcase draft")
	ErrStateConflict   = errors.New("showcase product state conflict")
	ErrVersionConflict = errors.New("showcase version conflict")
)

type PublishItem struct {
	SlotNo    int64 `json:"slot_no"`
	ProductID int64 `json:"product_id"`
}

type PublishInput struct {
	ExpectedVersion int64         `json:"expected_version"`
	Items           []PublishItem `json:"items"`
}

type ProductState struct {
	ProductID      int64
	ProductExists  bool
	ProductStatus  int64
	MerchantID     int64
	MerchantExists bool
	MerchantStatus int64
	StockAvailable int64
}

type SlotState struct {
	ProductExists  bool
	ProductStatus  int64
	MerchantExists bool
	MerchantStatus int64
	StockAvailable int64
}

type Slot struct {
	SlotNo        int64
	ProductID     int64
	Empty         bool
	Valid         bool
	InvalidReason string
}

type Layout struct {
	Version     int64
	OperatorID  int64
	PublishTime string
	Items       []Slot
}

type PublishSession interface {
	CurrentVersion(context.Context) (int64, error)
	ProductStates(context.Context, []int64) (map[int64]ProductState, error)
	Replace(context.Context, int64, []PublishItem) error
}

type Repository interface {
	Load(context.Context) (Layout, error)
	WithPublishSession(context.Context, func(PublishSession) error) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Load(ctx context.Context) (Layout, error) { return s.repository.Load(ctx) }

func (s *Service) Publish(ctx context.Context, operatorID int64, input PublishInput) (int64, error) {
	if input.ExpectedVersion <= 0 {
		return 0, fmt.Errorf("%w: expected_version must be positive", ErrInvalidDraft)
	}
	if err := ValidateDraftShape(input.Items); err != nil {
		return 0, err
	}
	var newVersion int64
	err := s.repository.WithPublishSession(ctx, func(session PublishSession) error {
		currentVersion, err := session.CurrentVersion(ctx)
		if err != nil {
			return err
		}
		if currentVersion != input.ExpectedVersion {
			return fmt.Errorf("%w: current_version=%d", ErrVersionConflict, currentVersion)
		}
		productIDs := make([]int64, 0, len(input.Items))
		for _, item := range input.Items {
			productIDs = append(productIDs, item.ProductID)
		}
		states, err := session.ProductStates(ctx, productIDs)
		if err != nil {
			return err
		}
		if err := ValidateDraft(input.Items, states); err != nil {
			return err
		}
		if err := session.Replace(ctx, operatorID, input.Items); err != nil {
			return err
		}
		newVersion = currentVersion + 1
		return nil
	})
	return newVersion, err
}

func InvalidReason(state SlotState) string {
	switch {
	case !state.ProductExists:
		return "product_not_found"
	case state.ProductStatus != 1:
		return "product_inactive"
	case !state.MerchantExists:
		return "merchant_not_found"
	case state.MerchantStatus != 1:
		return "merchant_inactive"
	case state.StockAvailable <= 0:
		return "out_of_stock"
	default:
		return ""
	}
}

func ValidateDraft(items []PublishItem, states map[int64]ProductState) error {
	if err := ValidateDraftShape(items); err != nil {
		return err
	}
	merchantCounts := make(map[int64]int)
	for _, item := range items {
		state, exists := states[item.ProductID]
		if !exists {
			state = ProductState{ProductID: item.ProductID}
		}
		reason := InvalidReason(SlotState{
			ProductExists: state.ProductExists, ProductStatus: state.ProductStatus,
			MerchantExists: state.MerchantExists, MerchantStatus: state.MerchantStatus,
			StockAvailable: state.StockAvailable,
		})
		if reason != "" {
			return fmt.Errorf("%w: slot=%d product=%d reason=%s", ErrStateConflict, item.SlotNo, item.ProductID, reason)
		}
		merchantCounts[state.MerchantID]++
		if merchantCounts[state.MerchantID] > 2 {
			return fmt.Errorf("%w: merchant %d exceeds two slots", ErrInvalidDraft, state.MerchantID)
		}
	}
	return nil
}

func ValidateDraftShape(items []PublishItem) error {
	if len(items) > 12 {
		return fmt.Errorf("%w: at most 12 items are allowed", ErrInvalidDraft)
	}
	slots := make(map[int64]struct{}, len(items))
	products := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.SlotNo < 1 || item.SlotNo > 12 || item.ProductID <= 0 {
			return fmt.Errorf("%w: slot and product must be positive and in range", ErrInvalidDraft)
		}
		if _, exists := slots[item.SlotNo]; exists {
			return fmt.Errorf("%w: duplicate slot %d", ErrInvalidDraft, item.SlotNo)
		}
		slots[item.SlotNo] = struct{}{}
		if _, exists := products[item.ProductID]; exists {
			return fmt.Errorf("%w: duplicate product %d", ErrInvalidDraft, item.ProductID)
		}
		products[item.ProductID] = struct{}{}
	}
	return nil
}

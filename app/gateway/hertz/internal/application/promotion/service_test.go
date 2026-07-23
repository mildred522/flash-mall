package promotion

import (
	"context"
	"testing"
	"time"
)

func TestCreateRejectsDiscountAboveProductSalePrice(t *testing.T) {
	repo := &fakeRepository{salePrice: 1000, productFound: true}
	service := NewService(repo, fixedClock)

	_, err := service.Create(context.Background(), CreateInput{ProductID: 7, DiscountValue: 1001})
	fault := requireFault(t, err)
	if fault.Reason != ReasonInvalidDiscount {
		t.Fatalf("reason = %q, want %q", fault.Reason, ReasonInvalidDiscount)
	}
	if repo.inserted {
		t.Fatal("invalid promotion must not be inserted")
	}
}

func TestCreateRejectsOverlappingActiveWindow(t *testing.T) {
	repo := &fakeRepository{salePrice: 1000, productFound: true, conflict: true}
	service := NewService(repo, fixedClock)

	_, err := service.Create(context.Background(), CreateInput{ProductID: 7, DiscountValue: 900, Status: StatusActive})
	fault := requireFault(t, err)
	if fault.Reason != ReasonWindowConflict {
		t.Fatalf("reason = %q, want %q", fault.Reason, ReasonWindowConflict)
	}
}

func TestUpdateReturnsBothAffectedProducts(t *testing.T) {
	repo := &fakeRepository{
		salePrice:    1000,
		productFound: true,
		record:       Record{ID: 5, ProductID: 7, DiscountValue: 800, Status: StatusActive},
		recordFound:  true,
		updated:      true,
	}
	service := NewService(repo, fixedClock)
	productID := int64(9)

	result, err := service.Update(context.Background(), UpdateInput{PromotionID: 5, ProductID: &productID})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(result.AffectedProductIDs) != 2 || result.AffectedProductIDs[0] != 7 || result.AffectedProductIDs[1] != 9 {
		t.Fatalf("affected products = %v, want [7 9]", result.AffectedProductIDs)
	}
}

func TestUpdateRejectsMissingPromotion(t *testing.T) {
	service := NewService(&fakeRepository{}, fixedClock)
	status := int64(StatusInactive)

	_, err := service.Update(context.Background(), UpdateInput{PromotionID: 99, Status: &status})
	fault := requireFault(t, err)
	if fault.Reason != ReasonPromotionNotFound {
		t.Fatalf("reason = %q, want %q", fault.Reason, ReasonPromotionNotFound)
	}
}

func TestWindowAffectedProductIDsNormalizesLimits(t *testing.T) {
	repository := &fakeRepository{affectedProductIDs: []int64{8, 9}}
	ids, err := NewService(repository, fixedClock).WindowAffectedProductIDs(context.Background(), 0, 0)
	if err != nil || len(ids) != 2 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	if repository.windowMinutes != 120 || repository.limit != 1000 {
		t.Fatalf("window=%d limit=%d", repository.windowMinutes, repository.limit)
	}
}

func fixedClock() time.Time { return time.Date(2026, 7, 19, 12, 0, 0, 0, time.Local) }

func requireFault(t *testing.T, err error) *Fault {
	t.Helper()
	fault, ok := AsFault(err)
	if !ok {
		t.Fatalf("error = %v, want promotion fault", err)
	}
	return fault
}

type fakeRepository struct {
	salePrice          int64
	productFound       bool
	conflict           bool
	record             Record
	recordFound        bool
	inserted           bool
	updated            bool
	affectedProductIDs []int64
	windowMinutes      int64
	limit              int64
}

func (f *fakeRepository) List(context.Context, ListQuery) ([]Record, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepository) Find(context.Context, int64) (Record, bool, error) {
	return f.record, f.recordFound, nil
}

func (f *fakeRepository) ProductSalePrice(context.Context, int64) (int64, bool, error) {
	return f.salePrice, f.productFound, nil
}

func (f *fakeRepository) HasActiveConflict(context.Context, int64, int64, *time.Time, *time.Time) (bool, error) {
	return f.conflict, nil
}

func (f *fakeRepository) Insert(context.Context, Record) (int64, error) {
	f.inserted = true
	return 11, nil
}

func (f *fakeRepository) Update(context.Context, int64, Changes) (bool, error) {
	return f.updated, nil
}

func (f *fakeRepository) WindowAffectedProductIDs(_ context.Context, _ time.Time, windowMinutes, limit int64) ([]int64, error) {
	f.windowMinutes, f.limit = windowMinutes, limit
	return f.affectedProductIDs, nil
}

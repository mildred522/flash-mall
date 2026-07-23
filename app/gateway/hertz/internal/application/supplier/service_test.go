package supplier

import (
	"context"
	"testing"
)

func TestCreateTrimsNameAndDefaultsToActive(t *testing.T) {
	repository := &fakeRepository{insertID: 41}
	service := NewService(repository)

	result, err := service.Create(context.Background(), CreateInput{Name: "  山岚供应链  "})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	if result.SupplierID != 41 {
		t.Fatalf("supplier id = %d, want 41", result.SupplierID)
	}
	if repository.inserted.Name != "山岚供应链" || repository.inserted.Status != StatusActive {
		t.Fatalf("inserted = %+v, want trimmed active supplier", repository.inserted)
	}
}

func TestUpdateRejectsInactiveSupplierWithActiveProducts(t *testing.T) {
	repository := &fakeRepository{updateResult: UpdateResult{Found: true, HasActiveProducts: true}}
	service := NewService(repository)
	status := int64(StatusInactive)

	err := service.Update(context.Background(), UpdateInput{SupplierID: 7, Status: &status})
	fault := requireFault(t, err)
	if fault.Reason != ReasonHasActiveProducts {
		t.Fatalf("reason = %q, want %q", fault.Reason, ReasonHasActiveProducts)
	}
}

func TestUpdateRejectsMissingSupplier(t *testing.T) {
	service := NewService(&fakeRepository{})

	err := service.Update(context.Background(), UpdateInput{SupplierID: 99, Name: "新名称"})
	fault := requireFault(t, err)
	if fault.Reason != ReasonSupplierNotFound {
		t.Fatalf("reason = %q, want %q", fault.Reason, ReasonSupplierNotFound)
	}
}

func requireFault(t *testing.T, err error) *Fault {
	t.Helper()
	fault, ok := AsFault(err)
	if !ok {
		t.Fatalf("error = %v, want supplier fault", err)
	}
	return fault
}

type fakeRepository struct {
	insertID     int64
	inserted     Record
	updateResult UpdateResult
}

func (f *fakeRepository) List(context.Context, ListQuery) ([]Record, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepository) Find(context.Context, int64) (Record, bool, error) {
	return Record{}, false, nil
}

func (f *fakeRepository) Insert(_ context.Context, record Record) (int64, error) {
	f.inserted = record
	return f.insertID, nil
}

func (f *fakeRepository) Update(context.Context, int64, Changes) (UpdateResult, error) {
	return f.updateResult, nil
}

package productcommand

import (
	"context"
	"testing"
)

func TestCreateNormalizesInputBeforePersistence(t *testing.T) {
	repository := &fakeRepository{createResult: CreateResult{ProductID: 101}}
	service := NewService(repository)

	result, err := service.Create(context.Background(), CreateInput{
		Name: "  山岚曲奇  ", MerchantID: 7, OriginPriceFen: 2000, SalePriceFen: 1600, SupplierID: 3,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if result.ProductID != 101 || repository.created.Name != "山岚曲奇" || repository.created.Status != StatusActive {
		t.Fatalf("result = %+v, created = %+v", result, repository.created)
	}
}

func TestCreateRejectsSalePriceAboveOriginPrice(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository)

	_, err := service.Create(context.Background(), CreateInput{
		Name: "商品", MerchantID: 7, OriginPriceFen: 1000, SalePriceFen: 1001, SupplierID: 3,
	})
	fault := requireFault(t, err)
	if fault.Reason != ReasonInvalidPrice || repository.createCalled {
		t.Fatalf("fault = %+v, createCalled = %v", fault, repository.createCalled)
	}
}

func TestUpdateMapsMerchantScopeMissToProductNotFound(t *testing.T) {
	repository := &fakeRepository{updateResult: UpdateResult{Found: false}}
	service := NewService(repository)
	name := "新名称"

	err := service.Update(context.Background(), UpdateInput{ProductID: 100, MerchantID: 7, Name: name})
	fault := requireFault(t, err)
	if fault.Reason != ReasonProductNotFound || repository.updated.MerchantID != 7 {
		t.Fatalf("fault = %+v, command = %+v", fault, repository.updated)
	}
}

func requireFault(t *testing.T, err error) *Fault {
	t.Helper()
	fault, ok := AsFault(err)
	if !ok {
		t.Fatalf("error = %v, want product command fault", err)
	}
	return fault
}

type fakeRepository struct {
	createCalled bool
	created      CreateRecord
	createResult CreateResult
	updated      UpdateCommand
	updateResult UpdateResult
}

func (f *fakeRepository) Create(_ context.Context, record CreateRecord) (CreateResult, error) {
	f.createCalled = true
	f.created = record
	return f.createResult, nil
}

func (f *fakeRepository) Update(_ context.Context, command UpdateCommand) (UpdateResult, error) {
	f.updated = command
	return f.updateResult, nil
}

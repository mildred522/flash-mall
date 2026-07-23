package useraddress

import (
	"context"
	"testing"
)

type repositoryStub struct {
	input  UpsertInput
	result UpsertRecord
}

func (r *repositoryStub) List(context.Context, int64) ([]Address, error) { return nil, nil }

func (r *repositoryStub) Upsert(_ context.Context, input UpsertInput) (UpsertRecord, error) {
	r.input = input
	return r.result, nil
}

func TestUpsertNormalizesRequiredFields(t *testing.T) {
	repository := &repositoryStub{result: UpsertRecord{AddressID: 8, Found: true}}
	result, err := NewService(repository).Upsert(context.Background(), UpsertInput{
		UserID: 7, ReceiverName: " 收件人 ", ReceiverPhone: " 13800000003 ",
		Province: " 浙江 ", City: " 杭州 ", District: " 西湖 ", Detail: " 文三路 1 号 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.input.ReceiverName != "收件人" || repository.input.Province != "浙江" ||
		repository.input.Detail != "文三路 1 号" || result.AddressID != 8 {
		t.Fatalf("input = %+v, result = %+v", repository.input, result)
	}
}

func TestUpsertRejectsMissingRequiredFieldBeforePersistence(t *testing.T) {
	repository := &repositoryStub{}
	_, err := NewService(repository).Upsert(context.Background(), UpsertInput{UserID: 7, ReceiverName: "收件人"})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonInvalidArgument {
		t.Fatalf("expected invalid argument fault, got %v", err)
	}
	if repository.input.UserID != 0 {
		t.Fatal("repository must not run for invalid input")
	}
}

func TestUpsertMapsMissingOwnedAddress(t *testing.T) {
	repository := &repositoryStub{result: UpsertRecord{Found: false}}
	_, err := NewService(repository).Upsert(context.Background(), UpsertInput{
		AddressID: 9, UserID: 7, ReceiverName: "收件人", ReceiverPhone: "13800000003", Detail: "文三路 1 号",
	})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonNotFound {
		t.Fatalf("expected not found fault, got %v", err)
	}
}

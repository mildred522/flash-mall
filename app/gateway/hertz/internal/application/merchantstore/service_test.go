package merchantstore

import (
	"context"
	"testing"
)

type repositoryStub struct {
	input  UpdateInput
	update UpdateRecord
}

func (r *repositoryStub) Profile(context.Context, int64) (Profile, bool, error) {
	return Profile{}, false, nil
}

func (r *repositoryStub) Update(_ context.Context, input UpdateInput) (UpdateRecord, error) {
	r.input = input
	return r.update, nil
}

func TestUpdateNormalizesAndAcceptsAllowedAssets(t *testing.T) {
	repository := &repositoryStub{update: UpdateRecord{Found: true}}
	err := NewService(repository).Update(context.Background(), UpdateInput{
		MerchantID: 7, LogoURL: " /uploads/stores/7/logo.png ",
		BannerURL: " https://cdn.example.com/banner.webp ", Description: " 简介 ", ExpectedVersion: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.input.LogoURL != "/uploads/stores/7/logo.png" || repository.input.Description != "简介" {
		t.Fatalf("input was not normalized: %+v", repository.input)
	}
}

func TestUpdateRejectsUnsafeAssetBeforePersistence(t *testing.T) {
	repository := &repositoryStub{}
	err := NewService(repository).Update(context.Background(), UpdateInput{
		MerchantID: 7, LogoURL: "javascript:alert(1)",
	})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonInvalidArgument {
		t.Fatalf("expected invalid argument fault, got %v", err)
	}
	if repository.input.MerchantID != 0 {
		t.Fatal("repository must not run for invalid input")
	}
}

func TestUpdateMapsVersionConflict(t *testing.T) {
	repository := &repositoryStub{update: UpdateRecord{Found: true, Rejection: RejectVersionConflict}}
	err := NewService(repository).Update(context.Background(), UpdateInput{MerchantID: 7, ExpectedVersion: 2})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonVersionConflict {
		t.Fatalf("expected version conflict fault, got %v", err)
	}
}

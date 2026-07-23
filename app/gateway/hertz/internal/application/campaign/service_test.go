package campaign

import (
	"context"
	"testing"
)

type repositoryStub struct {
	input UpsertInput
	id    int64
}

func (r *repositoryStub) List(context.Context) ([]Item, error) { return []Item{}, nil }

func (r *repositoryStub) Upsert(_ context.Context, input UpsertInput) (int64, error) {
	r.input = input
	return r.id, nil
}

func TestUpsertNormalizesInputAndDefaultsPerUserLimit(t *testing.T) {
	repository := &repositoryStub{id: 12}
	id, err := NewService(repository).Upsert(context.Background(), UpsertInput{
		ProductID: 8, Name: " 限时抢购 ", CampaignStock: 20,
		StartsAt: " 2026-07-22 10:00:00 ", EndsAt: " 2026-07-22 12:00:00 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != 12 || repository.input.Name != "限时抢购" || repository.input.PerUserLimit != 1 {
		t.Fatalf("unexpected result id=%d input=%+v", id, repository.input)
	}
	if repository.input.StartsAt != "2026-07-22 10:00:00" || repository.input.EndsAt != "2026-07-22 12:00:00" {
		t.Fatalf("window was not normalized: %+v", repository.input)
	}
}

func TestUpsertRejectsInvalidInputBeforePersistence(t *testing.T) {
	repository := &repositoryStub{}
	_, err := NewService(repository).Upsert(context.Background(), UpsertInput{Name: "campaign", CampaignStock: -1})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonInvalidArgument {
		t.Fatalf("expected invalid argument fault, got %v", err)
	}
	if repository.input.Name != "" {
		t.Fatal("repository must not run for invalid input")
	}
}

func TestListReturnsEmptySlice(t *testing.T) {
	items, err := NewService(&repositoryStub{}).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Fatal("items must be an empty JSON array, not nil")
	}
}

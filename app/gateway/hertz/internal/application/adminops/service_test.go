package adminops

import (
	"context"
	"testing"
)

type repositoryStub struct {
	eventQuery EventQuery
	retryID    string
}

func (r *repositoryStub) Dashboard(context.Context) (DashboardStats, error) {
	return DashboardStats{TotalOrders: 7}, nil
}

func (r *repositoryStub) Events(_ context.Context, query EventQuery) (EventList, error) {
	r.eventQuery = query
	return EventList{}, nil
}

func (r *repositoryStub) RetryEvent(_ context.Context, eventID string) error {
	r.retryID = eventID
	return nil
}

func TestEventsNormalizesQuery(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).Events(context.Background(), EventQuery{
		Page: 0, PageSize: 500, Status: 2, EventType: " paid ", AggregateID: " order-1 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.eventQuery.Page != 1 || repository.eventQuery.PageSize != 100 ||
		repository.eventQuery.EventType != "paid" || repository.eventQuery.AggregateID != "order-1" {
		t.Fatalf("query was not normalized: %+v", repository.eventQuery)
	}
	if result.Items == nil {
		t.Fatal("empty event list must serialize as []")
	}
}

func TestRetryEventRejectsBlankIDBeforePersistence(t *testing.T) {
	repository := &repositoryStub{}
	err := NewService(repository).RetryEvent(context.Background(), "  ")
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonInvalidArgument {
		t.Fatalf("expected invalid argument fault, got %v", err)
	}
	if repository.retryID != "" {
		t.Fatal("repository must not run for blank event id")
	}
}

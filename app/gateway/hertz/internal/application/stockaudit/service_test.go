package stockaudit

import (
	"context"
	"testing"
)

type repositoryStub struct{ query Query }

func (r *repositoryStub) List(_ context.Context, query Query) ([]Item, int64, error) {
	r.query = query
	return nil, 0, nil
}

func TestListNormalizesPaginationAndFilters(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).List(context.Background(), Query{
		Page: -2, PageSize: 101, OrderID: " o-1 ", ChangeType: " reserve ", MerchantID: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.Page != 1 || repository.query.PageSize != 20 || repository.query.OrderID != "o-1" || repository.query.ChangeType != "RESERVE" {
		t.Fatalf("query=%+v", repository.query)
	}
	if result.Items == nil || result.Page != 1 || result.PageSize != 20 {
		t.Fatalf("result=%+v", result)
	}
}

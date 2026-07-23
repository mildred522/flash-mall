package reconciliation

import (
	"context"
	"testing"
)

type repositoryStub struct{ query Query }

func (r *repositoryStub) List(_ context.Context, query Query) (List, error) {
	r.query = query
	return List{}, nil
}

func (r *repositoryStub) Scan(context.Context) (int64, error) { return 3, nil }

func TestListNormalizesFiltersAndPagination(t *testing.T) {
	repository := &repositoryStub{}
	result, err := NewService(repository).List(context.Background(), Query{
		Page: 0, PageSize: 500, Status: 0, IssueType: " mismatch ", OrderID: " order-1 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.Page != 1 || repository.query.PageSize != 100 ||
		repository.query.IssueType != "mismatch" || repository.query.OrderID != "order-1" {
		t.Fatalf("query was not normalized: %+v", repository.query)
	}
	if result.Items == nil {
		t.Fatal("empty reconciliation list must serialize as []")
	}
}

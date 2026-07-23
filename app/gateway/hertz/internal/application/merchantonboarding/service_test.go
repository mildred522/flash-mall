package merchantonboarding

import (
	"context"
	"testing"
)

type repositoryStub struct {
	submitInput SubmitInput
	submit      SubmitResult
	auditInput  AuditInput
	audit       AuditResult
}

func (r *repositoryStub) Submit(_ context.Context, input SubmitInput) (SubmitResult, error) {
	r.submitInput = input
	return r.submit, nil
}

func (r *repositoryStub) List(_ context.Context, _ ListQuery) (ListResult, error) {
	return ListResult{}, nil
}

func (r *repositoryStub) Audit(_ context.Context, input AuditInput) (AuditResult, error) {
	r.auditInput = input
	return r.audit, nil
}

func TestSubmitNormalizesInput(t *testing.T) {
	repository := &repositoryStub{submit: SubmitResult{ApplyID: 12, Status: StatusPending}}
	result, err := NewService(repository).Submit(context.Background(), SubmitInput{
		UserID: 9, MerchantName: "  山岚商店  ", ContactPhone: " 13800000003 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.submitInput.MerchantName != "山岚商店" || repository.submitInput.ContactPhone != "13800000003" {
		t.Fatalf("input was not normalized: %+v", repository.submitInput)
	}
	if result.ApplyID != 12 || result.Status != StatusPending {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubmitRejectsInvalidPhoneBeforePersistence(t *testing.T) {
	repository := &repositoryStub{}
	_, err := NewService(repository).Submit(context.Background(), SubmitInput{
		UserID: 9, MerchantName: "山岚商店", ContactPhone: "123",
	})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonInvalidArgument {
		t.Fatalf("expected invalid argument fault, got %v", err)
	}
	if repository.submitInput.UserID != 0 {
		t.Fatal("repository must not run for invalid input")
	}
}

func TestAuditRejectsEmptyRejectionRemarkBeforePersistence(t *testing.T) {
	repository := &repositoryStub{}
	_, err := NewService(repository).Audit(context.Background(), AuditInput{ApplyID: 12, Approve: false})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonInvalidArgument {
		t.Fatalf("expected invalid argument fault, got %v", err)
	}
	if repository.auditInput.ApplyID != 0 {
		t.Fatal("repository must not run for invalid input")
	}
}

func TestAuditMapsAlreadyAuditedRejection(t *testing.T) {
	repository := &repositoryStub{audit: AuditResult{Rejection: RejectAlreadyAudited}}
	_, err := NewService(repository).Audit(context.Background(), AuditInput{ApplyID: 12, Approve: true})
	fault, ok := AsFault(err)
	if !ok || fault.Reason != ReasonAlreadyAudited {
		t.Fatalf("expected already audited fault, got %v", err)
	}
}

func TestListNormalizesPaginationAndStatusText(t *testing.T) {
	repository := &listRepositoryStub{}
	result, err := NewService(repository).List(context.Background(), ListQuery{Status: 0, Page: 0, PageSize: 500})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.Page != 1 || repository.query.PageSize != 100 {
		t.Fatalf("query was not normalized: %+v", repository.query)
	}
	if len(result.Items) != 1 || result.Items[0].StatusText != "pending" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

type listRepositoryStub struct{ query ListQuery }

func (r *listRepositoryStub) Submit(context.Context, SubmitInput) (SubmitResult, error) {
	return SubmitResult{}, nil
}

func (r *listRepositoryStub) List(_ context.Context, query ListQuery) (ListResult, error) {
	r.query = query
	return ListResult{Items: []Application{{Status: StatusPending}}, Page: query.Page, PageSize: query.PageSize}, nil
}

func (r *listRepositoryStub) Audit(context.Context, AuditInput) (AuditResult, error) {
	return AuditResult{}, nil
}

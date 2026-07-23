package merchantonboarding

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

var contactPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

type Fault struct {
	Reason  string
	Message string
}

func (f *Fault) Error() string { return f.Message }

func AsFault(err error) (*Fault, bool) {
	var fault *Fault
	return fault, errors.As(err, &fault)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Submit(ctx context.Context, input SubmitInput) (SubmitResult, error) {
	input.MerchantName = strings.TrimSpace(input.MerchantName)
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	if input.UserID <= 0 || input.MerchantName == "" {
		return SubmitResult{}, fault(ReasonInvalidArgument, "user_id and merchant_name are required")
	}
	if input.ContactPhone != "" && !contactPhonePattern.MatchString(input.ContactPhone) {
		return SubmitResult{}, fault(ReasonInvalidArgument, "contact_phone is invalid")
	}
	result, err := s.repository.Submit(ctx, input)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := rejectionFault(result.Rejection); err != nil {
		return SubmitResult{}, err
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	if query.Status < -1 || query.Status > StatusRejected {
		return ListResult{}, fault(ReasonInvalidArgument, "status must be -1, 0, 1 or 2")
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	result, err := s.repository.List(ctx, query)
	if result.Items == nil {
		result.Items = []Application{}
	}
	for index := range result.Items {
		result.Items[index].StatusText = StatusText(result.Items[index].Status)
	}
	return result, err
}

func (s *Service) Audit(ctx context.Context, input AuditInput) (AuditResult, error) {
	input.Remark = strings.TrimSpace(input.Remark)
	if input.ApplyID <= 0 {
		return AuditResult{}, fault(ReasonInvalidArgument, "apply_id is required")
	}
	if !input.Approve && input.Remark == "" {
		return AuditResult{}, fault(ReasonInvalidArgument, "remark is required when rejecting")
	}
	result, err := s.repository.Audit(ctx, input)
	if err != nil {
		return AuditResult{}, err
	}
	if err := rejectionFault(result.Rejection); err != nil {
		return AuditResult{}, err
	}
	return result, nil
}

func StatusText(status int64) string {
	switch status {
	case StatusPending:
		return "pending"
	case StatusApproved:
		return "approved"
	case StatusRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

func rejectionFault(rejection string) error {
	switch rejection {
	case "":
		return nil
	case RejectAlreadyActive:
		return fault(ReasonAlreadyActive, "active merchant already exists")
	case RejectNotFound:
		return fault(ReasonNotFound, "merchant application not found")
	case RejectAlreadyAudited:
		return fault(ReasonAlreadyAudited, "merchant application already audited with a different decision")
	default:
		return errors.New("unknown merchant onboarding rejection: " + rejection)
	}
}

func fault(reason, message string) error { return &Fault{Reason: reason, Message: message} }

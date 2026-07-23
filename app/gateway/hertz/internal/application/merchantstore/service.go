package merchantstore

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

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

func (s *Service) Profile(ctx context.Context, merchantID int64) (Profile, error) {
	if merchantID <= 0 {
		return Profile{}, fault(ReasonInvalidArgument, "merchant_id is required")
	}
	profile, found, err := s.repository.Profile(ctx, merchantID)
	if err != nil {
		return Profile{}, err
	}
	if !found {
		return Profile{}, fault(ReasonNotFound, "merchant store not found")
	}
	return profile, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) error {
	input, err := PrepareUpdate(input)
	if err != nil {
		return err
	}
	if input.MerchantID <= 0 {
		return fault(ReasonInvalidArgument, "merchant_id is required")
	}
	record, err := s.repository.Update(ctx, input)
	if err != nil {
		return err
	}
	if !record.Found {
		return fault(ReasonNotFound, "merchant store unavailable")
	}
	if record.Rejection == RejectVersionConflict {
		return fault(ReasonVersionConflict, "merchant store profile version conflict")
	}
	if record.Rejection != "" {
		return errors.New("unknown merchant store rejection: " + record.Rejection)
	}
	return nil
}

// PrepareUpdate normalizes and validates the user-controlled store profile
// payload before the handler performs any database-backed identity lookup.
func PrepareUpdate(input UpdateInput) (UpdateInput, error) {
	input.LogoURL = strings.TrimSpace(input.LogoURL)
	input.BannerURL = strings.TrimSpace(input.BannerURL)
	input.Description = strings.TrimSpace(input.Description)
	if input.ExpectedVersion < 0 {
		return UpdateInput{}, fault(ReasonInvalidArgument, "expected_version must be non-negative")
	}
	if utf8.RuneCountInString(input.Description) > 1000 {
		return UpdateInput{}, fault(ReasonInvalidArgument, "description must not exceed 1000 characters")
	}
	if !validAssetURL(input.LogoURL) || !validAssetURL(input.BannerURL) {
		return UpdateInput{}, fault(ReasonInvalidArgument, "store asset URL must use an allowed local path or HTTP(S)")
	}
	return input, nil
}

func validAssetURL(raw string) bool {
	if raw == "" || strings.HasPrefix(raw, "/uploads/stores/") || strings.HasPrefix(raw, "/products/") {
		return true
	}
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func fault(reason, message string) error { return &Fault{Reason: reason, Message: message} }

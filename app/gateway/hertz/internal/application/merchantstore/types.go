package merchantstore

import "context"

const (
	ReasonInvalidArgument = "invalid_argument"
	ReasonNotFound        = "not_found"
	ReasonVersionConflict = "version_conflict"
)

const RejectVersionConflict = "version_conflict"

type Profile struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	LogoURL      string `json:"logo_url"`
	BannerURL    string `json:"banner_url"`
	Description  string `json:"description"`
	Version      int64  `json:"version"`
}

type UpdateInput struct {
	MerchantID      int64  `json:"-"`
	LogoURL         string `json:"logo_url"`
	BannerURL       string `json:"banner_url"`
	Description     string `json:"description"`
	ExpectedVersion int64  `json:"expected_version"`
}

type UpdateRecord struct {
	Found     bool
	Version   int64
	Rejection string
}

type Repository interface {
	Profile(context.Context, int64) (Profile, bool, error)
	Update(context.Context, UpdateInput) (UpdateRecord, error)
}

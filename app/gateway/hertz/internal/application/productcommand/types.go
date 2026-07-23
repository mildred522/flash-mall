package productcommand

import "context"

const (
	StatusActive   int64 = 1
	StatusInactive int64 = 2
)

const (
	ReasonInvalidArgument  = "invalid_argument"
	ReasonInvalidPrice     = "invalid_price"
	ReasonProductNotFound  = "product_not_found"
	ReasonSupplierNotFound = "supplier_not_found"
	ReasonMerchantNotFound = "merchant_not_found"
)

const (
	RejectInvalidPrice     = "invalid_price"
	RejectSupplierNotFound = "supplier_not_found"
	RejectMerchantNotFound = "merchant_not_found"
)

type CreateInput struct {
	Name           string `json:"name"`
	ImageURL       string `json:"image_url,omitempty"`
	MerchantID     int64  `json:"merchant_id,omitempty"`
	OriginPriceFen int64  `json:"origin_price_fen"`
	SalePriceFen   int64  `json:"sale_price_fen"`
	StockAvailable int64  `json:"stock_available,omitempty"`
	SupplierID     int64  `json:"supplier_id,omitempty"`
	Status         int64  `json:"status,omitempty"`
}

type UpdateInput struct {
	ProductID      int64  `json:"product_id"`
	MerchantID     int64  `json:"-"`
	Name           string `json:"name,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	SalePriceFen   *int64 `json:"sale_price_fen,omitempty"`
	OriginPriceFen *int64 `json:"origin_price_fen,omitempty"`
	SupplierID     *int64 `json:"supplier_id,omitempty"`
	Status         *int64 `json:"status,omitempty"`
}

type CreateRecord = CreateInput
type UpdateCommand = UpdateInput

type CreateResult struct {
	ProductID int64
	Rejection string
}

type UpdateResult struct {
	Found     bool
	Rejection string
}

type MutationResult struct{ ProductID int64 }

type Repository interface {
	Create(context.Context, CreateRecord) (CreateResult, error)
	Update(context.Context, UpdateCommand) (UpdateResult, error)
}

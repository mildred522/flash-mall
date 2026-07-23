package useraddress

import "context"

const (
	ReasonInvalidArgument = "invalid_argument"
	ReasonNotFound        = "not_found"
)

type Address struct {
	AddressID     int64  `json:"address_id"`
	ReceiverName  string `json:"receiver_name"`
	ReceiverPhone string `json:"receiver_phone"`
	Province      string `json:"province"`
	City          string `json:"city"`
	District      string `json:"district"`
	Detail        string `json:"detail"`
	IsDefault     bool   `json:"is_default"`
}

type ListResult struct {
	Items []Address `json:"items"`
}

type UpsertInput struct {
	AddressID     int64  `json:"address_id,omitempty"`
	UserID        int64  `json:"-"`
	ReceiverName  string `json:"receiver_name"`
	ReceiverPhone string `json:"receiver_phone"`
	Province      string `json:"province,omitempty"`
	City          string `json:"city,omitempty"`
	District      string `json:"district,omitempty"`
	Detail        string `json:"detail"`
	IsDefault     bool   `json:"is_default,omitempty"`
}

type UpsertRecord struct {
	AddressID int64
	Found     bool
}

type UpsertResult struct {
	AddressID int64 `json:"address_id"`
}

type Repository interface {
	List(context.Context, int64) ([]Address, error)
	Upsert(context.Context, UpsertInput) (UpsertRecord, error)
}

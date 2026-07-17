package ports

import "context"

type CancelUserOrderCommand struct {
	OrderID string
	Reason  string
	UserID  int64
	Meta    RequestMeta
}

type CloseAdminOrderCommand struct {
	OrderID    string
	Reason     string
	OperatorID int64
	Meta       RequestMeta
}

type ShipAdminOrderCommand struct {
	OrderID    string
	OperatorID int64
	Meta       RequestMeta
}

type ShipMerchantOrderCommand struct {
	OrderID    string
	MerchantID int64
	Meta       RequestMeta
}

type ConfirmReceiptCommand struct {
	OrderID string
	UserID  int64
	Meta    RequestMeta
}

type OrderCommands interface {
	CancelUser(context.Context, CancelUserOrderCommand) error
	CloseAdmin(context.Context, CloseAdminOrderCommand) error
	ShipAdmin(context.Context, ShipAdminOrderCommand) error
	ShipMerchant(context.Context, ShipMerchantOrderCommand) error
	ConfirmReceipt(context.Context, ConfirmReceiptCommand) error
}

type InventoryReservationReleaser interface {
	ReleaseStock(ctx context.Context, orderID string, reason string, meta RequestMeta) error
}

package domain

type ReservationStatus string

const (
	ReservationReserved  ReservationStatus = "reserved"
	ReservationConfirmed ReservationStatus = "confirmed"
	ReservationReleased  ReservationStatus = "released"
)

type Reservation struct {
	OrderID   string
	ProductID int64
	Quantity  int64
	Status    ReservationStatus
}

type ReservationStats struct {
	Active     int64
	Expired    int64
	Processing int64
	DeadLetter int64
}

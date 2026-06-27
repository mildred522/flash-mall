package orderstatus

const (
	PendingPayment  int64 = 0
	Paid            int64 = 1
	Closed          int64 = 2
	Shipped         int64 = 3
	Completed       int64 = 4
	RefundRequested int64 = 5
	Refunded        int64 = 6
)

func Text(statusCode int64) string {
	switch statusCode {
	case PendingPayment:
		return "pending_payment"
	case Paid:
		return "paid"
	case Closed:
		return "closed"
	case Shipped:
		return "shipped"
	case Completed:
		return "completed"
	case RefundRequested:
		return "refund_requested"
	case Refunded:
		return "refunded"
	default:
		return "unknown"
	}
}

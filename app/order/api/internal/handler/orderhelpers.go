package handler

func orderStatusText(statusCode int64) string {
	switch statusCode {
	case 0:
		return "pending_payment"
	case 1:
		return "paid"
	case 2:
		return "closed"
	case 3:
		return "shipped"
	case 4:
		return "completed"
	case 5:
		return "refund_requested"
	case 6:
		return "refunded"
	case 7:
		return "refund_failed"
	default:
		return "unknown"
	}
}

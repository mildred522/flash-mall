package refundstatus

const (
	Requested int64 = iota
	Approved
	Success
	Rejected
	Failed
)

func Text(statusCode int64) string {
	switch statusCode {
	case Requested:
		return "requested"
	case Approved:
		return "approved"
	case Success:
		return "success"
	case Rejected:
		return "rejected"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

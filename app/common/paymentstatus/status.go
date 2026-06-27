package paymentstatus

const (
	Init    int64 = 0
	Success int64 = 1
	Failed  int64 = 2
	Closed  int64 = 3
)

func Text(statusCode int64) string {
	switch statusCode {
	case Init:
		return "INIT"
	case Success:
		return "SUCCESS"
	case Failed:
		return "FAILED"
	case Closed:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

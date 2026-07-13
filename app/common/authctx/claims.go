package authctx

const (
	RoleUser     = "user"
	RoleMerchant = "merchant"
	RoleAdmin    = "admin"
)

// Identity is the normalized caller identity shared by gateway and services.
type Identity struct {
	UserID     int64
	Phone      string
	Role       string
	IsAdmin    bool
	MerchantID int64
	RequestID  string
}

func (i Identity) HasMerchant() bool {
	return i.MerchantID > 0
}

func (i Identity) CanAdmin() bool {
	return i.IsAdmin || i.Role == RoleAdmin
}

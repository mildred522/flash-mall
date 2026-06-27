package authstore

import "time"

type AuthStore interface {
	IssueCode(phone, scene string, ttlSeconds int64) (string, time.Time, error)
	ConsumeCode(phone, scene, code string, maxAttempts int64) error
	ResetCode(phone, scene string) error
	CreateUser(phone, displayName, password string) (*User, error)
	Authenticate(userID int64, phone, password string) (*User, error)
	GetUserByPhone(phone string) (*User, error)
	GetUserByID(userID int64) (*User, error)
	GetUserByIDAnyStatus(userID int64) (*User, error)
	ListAllUsers() []*User
	ListUsers(page, pageSize, status int64, role, keyword string) ([]*User, int64, error)
	SetUserStatus(userID int64, status int64) (*User, error)
	GetActiveSession(sessionID string) (*Session, error)
	CreateSession(userID int64, ttlSeconds int64) (string, string, error)
	CreateSessionForDevice(userID int64, deviceType string, ttlSeconds int64) (string, string, error)
	RefreshSession(refreshToken string, ttlSeconds int64) (*Session, string, error)
	Logout(refreshToken string) error
	LogoutAll(userID int64)
	UpdatePassword(phone, newPassword string) (*User, error)
}

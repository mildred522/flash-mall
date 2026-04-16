package authstore

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"flash-mall/app/auth/api/internal/sessionstate"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists           = errors.New("user already exists")
	ErrUserNotFound         = errors.New("user not found")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidCode          = errors.New("invalid verification code")
	ErrRefreshTokenInvalid  = errors.New("refresh token invalid")
	ErrRefreshTokenReplayed = errors.New("refresh token replayed")
)

type User struct {
	ID             int64
	Phone          string
	DisplayName    string
	PasswordHash   string
	SessionVersion int64
}

type verifyCode struct {
	Code      string
	ExpiresAt time.Time
	Attempts  int64
}

type Session struct {
	ID                       string
	UserID                   int64
	DeviceType               string
	SessionVersion           int64
	RefreshTokenHash         string
	PreviousRefreshTokenHash string
	RefreshFamilySecret      string
	RefreshGeneration        int64
	ExpiresAt                time.Time
	Revoked                  bool
}

type Store struct {
	mu             sync.RWMutex
	stateStore     sessionstate.StateStore
	nextUserID     int64
	usersByID      map[int64]*User
	usersByPhone   map[string]*User
	codes          map[string]verifyCode
	sessions       map[string]*Session
	refreshIndex   map[string]string
	userSessionIDs map[int64]map[string]struct{}
}

func NewStore(demoPassword string) *Store {
	return NewStoreWithState(demoPassword, nil)
}

func NewStoreWithState(demoPassword string, stateStore sessionstate.StateStore) *Store {
	s := &Store{
		stateStore:     stateStore,
		nextUserID:     1002,
		usersByID:      make(map[int64]*User),
		usersByPhone:   make(map[string]*User),
		codes:          make(map[string]verifyCode),
		sessions:       make(map[string]*Session),
		refreshIndex:   make(map[string]string),
		userSessionIDs: make(map[int64]map[string]struct{}),
	}
	_ = s.seedDemoUser(demoPassword)
	return s
}

func (s *Store) seedDemoUser(password string) error {
	if password == "" {
		password = "flashmall123"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := &User{
		ID:             1001,
		Phone:          "13800000001",
		DisplayName:    "Flash Mall 用户 1001",
		PasswordHash:   string(hash),
		SessionVersion: 1,
	}
	s.usersByID[user.ID] = user
	s.usersByPhone[user.Phone] = user
	s.syncUserVersionLocked(user.ID)
	return nil
}

func (s *Store) IssueCode(phone, scene string, ttlSeconds int64) (string, time.Time, error) {
	if phone == "" || scene == "" {
		return "", time.Time{}, ErrInvalidCode
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	code := "246810"
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[codeKey(phone, scene)] = verifyCode{
		Code:      code,
		ExpiresAt: expiresAt,
		Attempts:  0,
	}
	return code, expiresAt, nil
}

func (s *Store) ConsumeCode(phone, scene, code string, maxAttempts int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := codeKey(phone, scene)
	entry, ok := s.codes[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		delete(s.codes, key)
		return ErrInvalidCode
	}
	if entry.Code != code {
		entry.Attempts++
		if maxAttempts > 0 && entry.Attempts >= maxAttempts {
			delete(s.codes, key)
		} else {
			s.codes[key] = entry
		}
		return ErrInvalidCode
	}
	delete(s.codes, key)
	return nil
}

func (s *Store) ResetCode(phone, scene string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.codes, codeKey(phone, scene))
	return nil
}

func (s *Store) CreateUser(phone, displayName, password string) (*User, error) {
	if phone == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.usersByPhone[phone]; exists {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = fmt.Sprintf("闪购用户 %d", s.nextUserID)
	}

	user := &User{
		ID:             s.nextUserID,
		Phone:          phone,
		DisplayName:    displayName,
		PasswordHash:   string(hash),
		SessionVersion: 1,
	}
	s.nextUserID++
	s.usersByID[user.ID] = user
	s.usersByPhone[user.Phone] = user
	s.syncUserVersionLocked(user.ID)
	return user, nil
}

func (s *Store) Authenticate(userID int64, phone, password string) (*User, error) {
	s.mu.RLock()
	var user *User
	if phone != "" {
		user = s.usersByPhone[phone]
	} else {
		user = s.usersByID[userID]
	}
	s.mu.RUnlock()

	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Store) GetUserByPhone(phone string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByPhone[phone]
	return user, ok
}

func (s *Store) GetUserByID(userID int64) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByID[userID]
	return user, ok
}

func (s *Store) GetActiveSession(sessionID string) (*Session, bool) {
	if sessionID == "" {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok || session == nil || session.Revoked || time.Now().After(session.ExpiresAt) {
		return nil, false
	}
	return cloneSession(session), true
}

func (s *Store) CreateSession(userID int64, ttlSeconds int64) (string, string, error) {
	return s.CreateSessionForDevice(userID, "web", ttlSeconds)
}

func (s *Store) CreateSessionForDevice(userID int64, deviceType string, ttlSeconds int64) (string, string, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 7 * 24 * 60 * 60
	}
	if deviceType == "" {
		deviceType = "web"
	}
	sessionID, err := randomToken()
	if err != nil {
		return "", "", err
	}
	refreshFamilySecret, err := randomSecret()
	if err != nil {
		return "", "", err
	}
	refreshToken := buildRefreshToken(sessionID, 1, refreshFamilySecret)
	refreshHash := hashToken(refreshToken)
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.usersByID[userID]
	if user == nil {
		return "", "", ErrUserNotFound
	}

	s.revokeUserDeviceSessionsLocked(userID, deviceType)

	session := &Session{
		ID:                  sessionID,
		UserID:              userID,
		DeviceType:          deviceType,
		SessionVersion:      user.SessionVersion,
		RefreshTokenHash:    refreshHash,
		RefreshFamilySecret: refreshFamilySecret,
		RefreshGeneration:   1,
		ExpiresAt:           expiresAt,
	}
	s.sessions[sessionID] = session
	s.refreshIndex[refreshHash] = sessionID
	if s.userSessionIDs[userID] == nil {
		s.userSessionIDs[userID] = make(map[string]struct{})
	}
	s.userSessionIDs[userID][sessionID] = struct{}{}
	s.syncSessionStateLocked(session)
	return sessionID, refreshToken, nil
}

func (s *Store) RefreshSession(refreshToken string, ttlSeconds int64) (*Session, string, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 7 * 24 * 60 * 60
	}

	sessionID, tokenGeneration, signature, ok := parseRefreshToken(refreshToken)
	if !ok {
		return nil, "", ErrRefreshTokenInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil || session.Revoked || time.Now().After(session.ExpiresAt) {
		return nil, "", ErrRefreshTokenInvalid
	}
	if !verifyRefreshTokenSignature(sessionID, tokenGeneration, signature, session.RefreshFamilySecret) {
		return nil, "", ErrRefreshTokenInvalid
	}
	if tokenGeneration < session.RefreshGeneration {
		s.revokeSessionLocked(sessionID)
		return nil, "", ErrRefreshTokenReplayed
	}
	if tokenGeneration > session.RefreshGeneration {
		return nil, "", ErrRefreshTokenInvalid
	}

	newGeneration := session.RefreshGeneration + 1
	newRefreshToken := buildRefreshToken(sessionID, newGeneration, session.RefreshFamilySecret)
	newRefreshHash := hashToken(newRefreshToken)
	delete(s.refreshIndex, session.RefreshTokenHash)
	session.PreviousRefreshTokenHash = session.RefreshTokenHash
	session.RefreshTokenHash = newRefreshHash
	session.RefreshGeneration = newGeneration
	session.ExpiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	s.refreshIndex[newRefreshHash] = sessionID
	s.syncSessionStateLocked(session)
	return cloneSession(session), newRefreshToken, nil
}

func (s *Store) Logout(refreshToken string) error {
	sessionID, tokenGeneration, signature, ok := parseRefreshToken(refreshToken)
	if !ok {
		return ErrRefreshTokenInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil || session.Revoked || time.Now().After(session.ExpiresAt) {
		return ErrRefreshTokenInvalid
	}
	if !verifyRefreshTokenSignature(sessionID, tokenGeneration, signature, session.RefreshFamilySecret) {
		return ErrRefreshTokenInvalid
	}
	if tokenGeneration < session.RefreshGeneration {
		s.revokeSessionLocked(sessionID)
		return ErrRefreshTokenReplayed
	}
	if tokenGeneration > session.RefreshGeneration {
		return ErrRefreshTokenInvalid
	}
	s.revokeSessionLocked(sessionID)
	return nil
}

func (s *Store) LogoutAll(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bumpUserSessionVersionLocked(userID)
	s.syncUserVersionLocked(userID)
	s.revokeUserSessionsLocked(userID)
}

func (s *Store) UpdatePassword(phone, newPassword string) (*User, error) {
	if phone == "" || newPassword == "" {
		return nil, ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.usersByPhone[phone]
	if !ok {
		return nil, ErrUserNotFound
	}
	user.PasswordHash = string(hash)
	s.bumpUserSessionVersionLocked(user.ID)
	s.syncUserVersionLocked(user.ID)
	s.revokeUserSessionsLocked(user.ID)
	return user, nil
}

func (s *Store) bumpUserSessionVersionLocked(userID int64) {
	user := s.usersByID[userID]
	if user == nil {
		return
	}
	user.SessionVersion++
	if user.SessionVersion <= 0 {
		user.SessionVersion = 1
	}
}

func (s *Store) revokeUserSessionsLocked(userID int64) {
	ids := s.userSessionIDs[userID]
	for sessionID := range ids {
		s.revokeSessionLocked(sessionID)
		delete(ids, sessionID)
	}
}

func (s *Store) revokeUserDeviceSessionsLocked(userID int64, deviceType string) {
	ids := s.userSessionIDs[userID]
	for sessionID := range ids {
		session := s.sessions[sessionID]
		if session == nil {
			delete(ids, sessionID)
			continue
		}
		if session.DeviceType != deviceType {
			continue
		}
		s.revokeSessionLocked(sessionID)
		delete(ids, sessionID)
	}
}

func (s *Store) deleteRefreshIndexLocked(session *Session) {
	if session == nil {
		return
	}
	if session.RefreshTokenHash != "" {
		delete(s.refreshIndex, session.RefreshTokenHash)
	}
}

func (s *Store) revokeSessionLocked(sessionID string) {
	session := s.sessions[sessionID]
	if session == nil {
		return
	}
	session.Revoked = true
	s.deleteRefreshIndexLocked(session)
	if ids := s.userSessionIDs[session.UserID]; ids != nil {
		delete(ids, sessionID)
	}
	s.deleteSessionStateLocked(sessionID)
}

func (s *Store) syncSessionStateLocked(session *Session) {
	if s.stateStore == nil || session == nil {
		return
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return
	}
	_ = s.stateStore.SaveSession(context.Background(), sessionstate.SessionSnapshot{
		SessionID:      session.ID,
		UserID:         session.UserID,
		DeviceType:     session.DeviceType,
		SessionVersion: session.SessionVersion,
	}, ttl)
}

func (s *Store) deleteSessionStateLocked(sessionID string) {
	if s.stateStore == nil || sessionID == "" {
		return
	}
	_ = s.stateStore.DeleteSession(context.Background(), sessionID)
}

func (s *Store) syncUserVersionLocked(userID int64) {
	if s.stateStore == nil {
		return
	}
	user := s.usersByID[userID]
	if user == nil {
		return
	}
	_ = s.stateStore.SetUserVersion(context.Background(), userID, user.SessionVersion)
}

func codeKey(phone, scene string) string {
	return scene + ":" + phone
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func cloneSession(session *Session) *Session {
	if session == nil {
		return nil
	}
	copy := *session
	return &copy
}

func buildRefreshToken(sessionID string, generation int64, familySecret string) string {
	return fmt.Sprintf("%s.%d.%s", sessionID, generation, signRefreshToken(sessionID, generation, familySecret))
}

func parseRefreshToken(token string) (string, int64, string, bool) {
	sessionID, generationPart, signature, ok := splitRefreshToken(token)
	if !ok {
		return "", 0, "", false
	}
	generation, err := parseGeneration(generationPart)
	if err != nil || generation <= 0 {
		return "", 0, "", false
	}
	return sessionID, generation, signature, true
}

func splitRefreshToken(token string) (string, string, string, bool) {
	sessionID, rest, ok := strings.Cut(token, ".")
	if !ok || sessionID == "" || rest == "" {
		return "", "", "", false
	}
	generation, signature, ok := strings.Cut(rest, ".")
	if !ok || generation == "" || signature == "" {
		return "", "", "", false
	}
	return sessionID, generation, signature, true
}

func parseGeneration(value string) (int64, error) {
	generation, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return generation, nil
}

func signRefreshToken(sessionID string, generation int64, familySecret string) string {
	mac := hmac.New(sha256.New, []byte(familySecret))
	_, _ = mac.Write([]byte(sessionID))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write([]byte(strconv.FormatInt(generation, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyRefreshTokenSignature(sessionID string, generation int64, signature, familySecret string) bool {
	expected := signRefreshToken(sessionID, generation, familySecret)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

package auth

import (
	"fmt"
	"time"

	"flash-mall/app/auth/api/internal/types"
	"github.com/golang-jwt/jwt/v4"
)

func issueLoginResp(secret string, accessTTLSeconds int64, userID int64, sessionVersion int64, sessionID, refreshToken, displayName, phone string) (*types.LoginResp, error) {
	if accessTTLSeconds <= 0 {
		accessTTLSeconds = 2 * 60 * 60
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(accessTTLSeconds) * time.Second)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":         userID,
		"sub":             fmt.Sprintf("%d", userID),
		"sid":             sessionID,
		"session_version": sessionVersion,
		"role":            "user",
		"iat":             now.Unix(),
		"exp":             expiresAt.Unix(),
	})
	accessToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, err
	}

	return &types.LoginResp{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt.Unix(),
		UserId:       userID,
		DisplayName:  displayName,
		Phone:        phone,
		RefreshToken: refreshToken,
	}, nil
}

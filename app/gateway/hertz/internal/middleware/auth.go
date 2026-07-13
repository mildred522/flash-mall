package middleware

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"flash-mall/app/common/apiresponse"
	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/golang-jwt/jwt/v4"
)

const authorizationHeader = "Authorization"

func RequireUser(secret string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, ok := parseBearerIdentity(ctx, c, secret)
		if !ok {
			return
		}
		c.Next(withLegacyClaims(authctx.WithIdentity(ctx, identity), identity))
	}
}

func OptionalIdentity(secret string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if bearerToken(string(c.GetHeader(authorizationHeader))) == "" {
			c.Next(ctx)
			return
		}
		identity, ok := parseBearerIdentity(ctx, c, secret)
		if !ok {
			return
		}
		c.Next(withLegacyClaims(authctx.WithIdentity(ctx, identity), identity))
	}
}

func RequireAdmin(secret string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, ok := parseBearerIdentity(ctx, c, secret)
		if !ok {
			return
		}
		if !identity.CanAdmin() {
			writeAuthError(ctx, c, consts.StatusForbidden, apperror.New(apperror.CodeForbidden, "admin access required"))
			return
		}
		c.Next(withLegacyClaims(authctx.WithIdentity(ctx, identity), identity))
	}
}

func RequireMerchant(secret string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, ok := parseBearerIdentity(ctx, c, secret)
		if !ok {
			return
		}
		// Merchant ownership is stored in merchant_user. The middleware only authenticates
		// and carries merchant_id; route handlers verify membership against storage.
		identity.MerchantID = parseMerchantID(c)
		c.Next(withLegacyClaims(authctx.WithIdentity(ctx, identity), identity))
	}
}

func parseBearerIdentity(ctx context.Context, c *app.RequestContext, secret string) (authctx.Identity, bool) {
	if strings.TrimSpace(secret) == "" {
		writeAuthError(ctx, c, consts.StatusInternalServerError, apperror.New(apperror.CodeInternal, "jwt secret is not configured"))
		return authctx.Identity{}, false
	}
	rawToken := bearerToken(string(c.GetHeader(authorizationHeader)))
	if rawToken == "" {
		writeAuthError(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "missing bearer token"))
		return authctx.Identity{}, false
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperror.New(apperror.CodeUnauthorized, "invalid token signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || token == nil || !token.Valid {
		writeAuthError(ctx, c, consts.StatusUnauthorized, apperror.Wrap(apperror.CodeUnauthorized, "invalid bearer token", err))
		return authctx.Identity{}, false
	}

	userID, ok := claimInt64(claims["user_id"])
	if !ok {
		userID, ok = claimInt64(claims["sub"])
	}
	if !ok || userID <= 0 {
		writeAuthError(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "missing user subject in jwt"))
		return authctx.Identity{}, false
	}

	role, _ := claims["role"].(string)
	identity := authctx.Identity{
		UserID:     userID,
		Phone:      claimString(claims["phone"]),
		Role:       strings.ToLower(strings.TrimSpace(role)),
		IsAdmin:    strings.EqualFold(role, authctx.RoleAdmin),
		MerchantID: parseMerchantID(c),
		RequestID:  tracectx.RequestIDFrom(ctx),
	}
	if identity.Role == "" {
		identity.Role = authctx.RoleUser
	}
	return identity, true
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return header
}

func writeAuthError(ctx context.Context, c *app.RequestContext, statusCode int, err error) {
	c.AbortWithStatusJSON(statusCode, apiresponse.Fail(tracectx.RequestIDFrom(ctx), err))
}

func withLegacyClaims(ctx context.Context, identity authctx.Identity) context.Context {
	ctx = context.WithValue(ctx, "user_id", identity.UserID) //nolint:staticcheck // go-zero stores JWT claims under string keys.
	ctx = context.WithValue(ctx, "role", identity.Role)      //nolint:staticcheck // keep compatibility with existing handlers.
	if identity.MerchantID > 0 {
		ctx = context.WithValue(ctx, "merchant_id", identity.MerchantID) //nolint:staticcheck // keep compatibility with existing handlers.
	}
	return ctx
}

func claimInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case json.Number:
		id, err := v.Int64()
		return id, err == nil
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return id, err == nil
	default:
		return 0, false
	}
}

func claimString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func parseMerchantID(c *app.RequestContext) int64 {
	for _, raw := range []string{
		string(c.GetHeader(tracectx.HeaderMerchantID)),
		c.Query("merchant_id"),
	} {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err == nil && id > 0 {
			return id
		}
	}
	return 0
}

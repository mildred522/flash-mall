package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	errMerchantStoreVersionConflict = errors.New("merchant store profile version conflict")
	errMerchantStoreUnavailable     = errors.New("merchant store unavailable")
)

func MerchantStoreProfileHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		merchantID, err := selectedMerchantID(ctx, db, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		if err = ensureMerchantStoreProfileTable(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant store schema unavailable", err))
			return
		}
		profile, err := loadMerchantStoreProfile(ctx, db, merchantID)
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeMerchantNotFound, "merchant store not found"))
			return
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant store query failed", err))
			return
		}
		ok(ctx, c, profile)
	}
}

func MerchantStoreUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		var req merchantStoreUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid merchant store request"))
			return
		}
		req.LogoURL = strings.TrimSpace(req.LogoURL)
		req.BannerURL = strings.TrimSpace(req.BannerURL)
		req.Description = strings.TrimSpace(req.Description)
		if err := validateMerchantStoreUpdate(req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, err.Error()))
			return
		}
		db, err := orderDB(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
			return
		}
		merchantID, err := selectedMerchantID(ctx, db, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		if err = ensureMerchantStoreProfileTable(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant store schema unavailable", err))
			return
		}
		if _, err = saveMerchantStoreProfile(ctx, db, merchantID, req); errors.Is(err, errMerchantStoreVersionConflict) {
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeConflict, err.Error()))
			return
		} else if errors.Is(err, errMerchantStoreUnavailable) {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeMerchantNotFound, err.Error()))
			return
		} else if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant store update failed", err))
			return
		}
		profile, err := loadMerchantStoreProfile(ctx, db, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant store query failed", err))
			return
		}
		ok(ctx, c, profile)
	}
}

func loadMerchantStoreProfile(ctx context.Context, db *sql.DB, merchantID int64) (MerchantStoreProfile, error) {
	var profile MerchantStoreProfile
	err := db.QueryRowContext(ctx, `
SELECT m.id, m.name,
       COALESCE(p.logo_url, ''), COALESCE(p.banner_url, ''),
       COALESCE(p.description, ''), COALESCE(p.version, 0)
FROM mall_order.merchant m
LEFT JOIN mall_order.merchant_store_profile p ON p.merchant_id = m.id
WHERE m.id = ? AND m.status = 1`, merchantID).Scan(
		&profile.MerchantID,
		&profile.MerchantName,
		&profile.LogoURL,
		&profile.BannerURL,
		&profile.Description,
		&profile.Version,
	)
	return profile, err
}

func validateMerchantStoreUpdate(req merchantStoreUpdateReq) error {
	if req.ExpectedVersion < 0 {
		return errors.New("expected_version must be non-negative")
	}
	if utf8.RuneCountInString(req.Description) > 1000 {
		return errors.New("description must not exceed 1000 characters")
	}
	if err := validateMerchantStoreAssetURL(req.LogoURL); err != nil {
		return err
	}
	return validateMerchantStoreAssetURL(req.BannerURL)
}

func validateMerchantStoreAssetURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/uploads/stores/") || strings.HasPrefix(raw, "/products/") {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return errors.New("store asset URL must use an allowed local path or HTTP(S)")
	}
	return nil
}

func saveMerchantStoreProfile(ctx context.Context, db *sql.DB, merchantID int64, req merchantStoreUpdateReq) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var merchantStatus int64
	if err := tx.QueryRowContext(ctx, `SELECT status FROM mall_order.merchant WHERE id = ? FOR UPDATE`, merchantID).Scan(&merchantStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errMerchantStoreUnavailable
		}
		return 0, err
	}
	if merchantStatus != 1 {
		return 0, errMerchantStoreUnavailable
	}

	var currentVersion int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM mall_order.merchant_store_profile WHERE merchant_id = ? FOR UPDATE`, merchantID).Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		if req.ExpectedVersion != 0 {
			return 0, errMerchantStoreVersionConflict
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO mall_order.merchant_store_profile
(merchant_id, logo_url, banner_url, description, version)
VALUES (?, ?, ?, ?, 1)`, merchantID, req.LogoURL, req.BannerURL, req.Description); err != nil {
			return 0, err
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	if currentVersion != req.ExpectedVersion {
		return 0, errMerchantStoreVersionConflict
	}

	result, err := tx.ExecContext(ctx, `UPDATE mall_order.merchant_store_profile
SET logo_url = ?, banner_url = ?, description = ?, version = version + 1
WHERE merchant_id = ? AND version = ?`, req.LogoURL, req.BannerURL, req.Description, merchantID, currentVersion)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected != 1 {
		return 0, errMerchantStoreVersionConflict
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return currentVersion + 1, nil
}

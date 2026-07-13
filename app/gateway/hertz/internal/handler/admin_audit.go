package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	adminAuditResultSuccess = "success"
	adminAuditResultFail    = "fail"
)

const (
	adminAuditOrderShipped  = "admin_order_shipped"
	adminAuditOrderRefunded = "admin_order_refunded"
	adminAuditOrderClosed   = "admin_order_closed"

	adminAuditProductCreated       = "admin_product_created"
	adminAuditProductUpdated       = "admin_product_updated"
	adminAuditProductEnabled       = "admin_product_enabled"
	adminAuditProductDisabled      = "admin_product_disabled"
	adminAuditProductStockAdjusted = "admin_product_stock_adjusted"

	adminAuditSupplierCreated  = "admin_supplier_created"
	adminAuditSupplierUpdated  = "admin_supplier_updated"
	adminAuditSupplierEnabled  = "admin_supplier_enabled"
	adminAuditSupplierDisabled = "admin_supplier_disabled"

	adminAuditPromotionCreated  = "admin_promotion_created"
	adminAuditPromotionUpdated  = "admin_promotion_updated"
	adminAuditPromotionEnabled  = "admin_promotion_enabled"
	adminAuditPromotionDisabled = "admin_promotion_disabled"

	adminAuditHomepageShowcasePublished = "homepage_showcase.publish"
)

const (
	adminAuditReasonActiveSupplierNotFound     = "active_supplier_not_found"
	adminAuditReasonHasActiveProducts          = "has_active_products"
	adminAuditReasonInsufficientOrMissingStock = "insufficient_or_missing_bucket"
	adminAuditReasonInvalidDiscount            = "invalid_discount"
	adminAuditReasonInvalidPrice               = "invalid_price"
	adminAuditReasonInvalidStatus              = "invalid_status"
	adminAuditReasonInvalidWindow              = "invalid_window"
	adminAuditReasonNotFound                   = "not_found"
	adminAuditReasonNotPaidStatus              = "not_paid_status"
	adminAuditReasonProductNotFound            = "product_not_found"
	adminAuditReasonStatusChanged              = "status_changed"
	adminAuditReasonWindowConflict             = "window_conflict"
)

type adminAuditPayload struct {
	EventType string `json:"event_type"`
	Result    string `json:"result"`
	Subject   string `json:"subject,omitempty"`
}

func recordGatewayAdminAuditEvent(c *app.RequestContext, svcCtx *svc.ServiceContext, eventType, subject string) {
	recordGatewayAdminAuditEventResult(c, svcCtx, eventType, adminAuditResultSuccess, subject)
}

func recordGatewayAdminAuditFailure(c *app.RequestContext, svcCtx *svc.ServiceContext, eventType, subject string) {
	recordGatewayAdminAuditEventResult(c, svcCtx, eventType, adminAuditResultFail, subject)
}

func recordGatewayAdminAuditEventResult(c *app.RequestContext, svcCtx *svc.ServiceContext, eventType, result, subject string) {
	baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
	eventType = strings.TrimSpace(eventType)
	if baseURL == "" || eventType == "" {
		return
	}

	authz := string(c.GetHeader("Authorization"))
	userAgent := string(c.GetHeader("User-Agent"))
	forwardedFor := string(c.GetHeader("X-Forwarded-For"))
	realIP := string(c.GetHeader("X-Real-IP"))
	subject = strings.TrimSpace(subject)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		payload, err := json.Marshal(adminAuditPayload{
			EventType: eventType,
			Result:    result,
			Subject:   subject,
		})
		if err != nil {
			logx.Errorf("gateway admin audit marshal failed: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/admin/security/events/record", bytes.NewReader(payload))
		if err != nil {
			logx.Errorf("gateway admin audit request build failed: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
		if forwardedFor != "" {
			req.Header.Set("X-Forwarded-For", forwardedFor)
		}
		if realIP != "" {
			req.Header.Set("X-Real-IP", realIP)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logx.Errorf("gateway admin audit request failed: %v", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= http.StatusMultipleChoices {
			logx.Errorf("gateway admin audit request returned status=%d event_type=%s", resp.StatusCode, eventType)
		}
	}()
}

func supplierUpdateAuditEvent(status *int64) string {
	if status == nil {
		return adminAuditSupplierUpdated
	}
	if *status == 1 {
		return adminAuditSupplierEnabled
	}
	return adminAuditSupplierDisabled
}

func productUpdateAuditEvent(status *int64) string {
	if status == nil {
		return adminAuditProductUpdated
	}
	if *status == 1 {
		return adminAuditProductEnabled
	}
	return adminAuditProductDisabled
}

func promotionUpdateAuditEvent(status *int64) string {
	if status == nil {
		return adminAuditPromotionUpdated
	}
	if *status == 1 {
		return adminAuditPromotionEnabled
	}
	return adminAuditPromotionDisabled
}

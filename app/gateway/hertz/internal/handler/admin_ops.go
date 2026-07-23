package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/adminops"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errAdminOpsUnavailable = errors.New("admin ops service unavailable")

func AdminDashboardStatsHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if svcCtx.AdminOps == nil {
			failAdminOps(ctx, c, "dashboard stats query failed", errAdminOpsUnavailable)
			return
		}
		stats, err := svcCtx.AdminOps.Dashboard(ctx)
		if err != nil {
			failAdminOps(ctx, c, "dashboard stats query failed", err)
			return
		}
		stats.TotalUsers = fetchGatewayAdminUserTotal(ctx, c, svcCtx)
		ok(ctx, c, stats)
	}
}

func AdminEventListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		query, err := adminEventQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		if svcCtx.AdminOps == nil {
			failAdminOps(ctx, c, "event query failed", errAdminOpsUnavailable)
			return
		}
		result, err := svcCtx.AdminOps.Events(ctx, query)
		if err != nil {
			failAdminOps(ctx, c, "event query failed", err)
			return
		}
		ok(ctx, c, result)
	}
}

func AdminEventRetryHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminEventRetryReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest,
				apperror.New(apperror.CodeInvalidArgument, "invalid event retry request"))
			return
		}
		if svcCtx.AdminOps == nil {
			failAdminOps(ctx, c, "event retry failed", errAdminOpsUnavailable)
			return
		}
		if err := svcCtx.AdminOps.RetryEvent(ctx, req.EventID); err != nil {
			failAdminOps(ctx, c, "event retry failed", err)
			return
		}
		ok(ctx, c, AdminEventRetryResp{EventID: strings.TrimSpace(req.EventID), Status: "pending"})
	}
}

func adminEventQueryFromRequest(c *app.RequestContext) (AdminEventListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminEventListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminEventListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminEventListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	return AdminEventListReq{
		Page: page, PageSize: pageSize, Status: status,
		EventType: c.Query("event_type"), AggregateID: c.Query("aggregate_id"),
	}, nil
}

func failAdminOps(ctx context.Context, c *app.RequestContext, operation string, err error) {
	if fault, ok := adminops.AsFault(err); ok {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
		return
	}
	fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, operation, err))
}

func fetchGatewayAdminUserTotal(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext) int64 {
	baseURL := strings.TrimRight(svcCtx.Config.AuthServiceBaseURL, "/")
	if baseURL == "" {
		return 0
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/api/admin/users?page=1&page_size=1", nil)
	if err != nil {
		return 0
	}
	for _, name := range []string{"Authorization", "User-Agent", "X-Forwarded-For", "X-Real-IP"} {
		copyHeaderIfPresent(c, req, name)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0
	}
	var payload struct {
		Total int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0
	}
	return payload.Total
}

func copyHeaderIfPresent(c *app.RequestContext, req *http.Request, name string) {
	if value := string(c.GetHeader(name)); value != "" {
		req.Header.Set(name, value)
	}
}

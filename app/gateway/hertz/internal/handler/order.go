package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/application/orderquery"
	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/dtm-labs/dtm/client/dtmgrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CreateOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req CreateOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order create request"))
			return
		}
		req.RequestID = strings.TrimSpace(req.RequestID)
		req.UserID = identity.UserID
		if len(req.RequestID) > 64 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "request_id must be <= 64 characters"))
			return
		}
		if req.ProductID <= 0 || req.Amount <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id and positive amount are required"))
			return
		}
		if req.ExpectedPriceFen < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "expected_price_fen must be non-negative"))
			return
		}
		if strings.TrimSpace(svcCtx.Config.DtmServer) == "" || strings.TrimSpace(svcCtx.Config.OrderRpcTarget) == "" {
			fail(ctx, c, consts.StatusBadGateway, apperror.New(apperror.CodeInternal, "order saga is not configured"))
			return
		}

		if req.RequestID != "" {
			if resp, found, err := loadCreateOrderRespByRequestID(ctx, svcCtx, req.RequestID, req.UserID); err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order idempotency lookup failed", err))
				return
			} else if found {
				ok(ctx, c, resp)
				return
			}
		}

		orderID := orderIDForRequest(req.RequestID)
		if err := submitCreateOrderSaga(svcCtx, req, orderID); err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}

		resp, err := loadCreateOrderRespByOrderID(ctx, svcCtx, orderID, req.UserID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func PayOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		var req PayOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid order pay request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}

		payment, err := loadUserPaymentOrder(ctx, svcCtx, req.OrderID, identity.UserID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		if !orderstatus.CanPay(payment.OrderStatus) && payment.OrderStatus != orderstatus.Paid {
			fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeOrderStatusInvalid, "order is not payable"))
			return
		}
		resp, err := buildPaymentIntentResp(c, svcCtx, payment, time.Now())
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func submitCreateOrderSaga(svcCtx *svc.ServiceContext, req CreateOrderReq, orderID string) error {
	gid := dtmgrpc.MustGenGid(svcCtx.Config.DtmServer)
	saga := dtmgrpc.NewSagaGrpc(svcCtx.Config.DtmServer, gid)
	saga.WaitResult = true
	if svcCtx.Config.DtmTimeoutToFailSeconds > 0 {
		saga.TimeoutToFail = svcCtx.Config.DtmTimeoutToFailSeconds
	}
	if svcCtx.Config.DtmRequestTimeoutSeconds > 0 {
		saga.WithGlobalTransRequestTimeout(svcCtx.Config.DtmRequestTimeoutSeconds)
	}
	if svcCtx.Config.DtmWaitResult {
		saga.WaitResult = true
	}

	orderRoute := strings.TrimRight(svcCtx.Config.OrderRpcTarget, "/") + "/order.Order"
	saga.Add(orderRoute+"/PreDeduct", orderRoute+"/PreDeductRollback", &orderpb.PreDeductReq{
		ProductId: req.ProductID,
		Amount:    req.Amount,
		OrderId:   orderID,
	})
	saga.Add(orderRoute+"/CreateOrder", orderRoute+"/CreateOrderRollback", &orderpb.CreateOrderReq{
		OrderId:          orderID,
		RequestId:        requestIDForOrder(req.RequestID, orderID),
		UserId:           req.UserID,
		ProductId:        req.ProductID,
		Amount:           req.Amount,
		ExpectedPriceFen: req.ExpectedPriceFen,
	})
	if err := saga.Submit(); err != nil {
		return status.Error(codes.Unavailable, "order system busy")
	}
	return nil
}

type userPaymentOrder = orderquery.PaymentOrder

func loadUserPaymentOrder(ctx context.Context, svcCtx *svc.ServiceContext, orderID string, userID int64) (userPaymentOrder, error) {
	queries, err := orderQueryService(svcCtx)
	if err != nil {
		return userPaymentOrder{}, err
	}
	return queries.Payment(ctx, orderID, userID)
}

func loadCreateOrderRespByRequestID(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID int64) (CreateOrderResp, bool, error) {
	queries, err := orderQueryService(svcCtx)
	if err != nil {
		return CreateOrderResp{}, false, err
	}
	return queries.CreateByRequest(ctx, requestID, userID)
}

func loadCreateOrderRespByOrderID(ctx context.Context, svcCtx *svc.ServiceContext, orderID string, userID int64) (CreateOrderResp, error) {
	queries, err := orderQueryService(svcCtx)
	if err != nil {
		return CreateOrderResp{}, err
	}
	return queries.CreateByOrder(ctx, orderID, userID)
}

func orderIDForRequest(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return requestID
	}
	return fmt.Sprintf("o-%d", time.Now().UnixNano())
}

func requestIDForOrder(requestID string, orderID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		return requestID
	}
	return orderID
}

func createOrderStatusCode(err error) int {
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return consts.StatusBadRequest
	case apperror.CodeUnauthorized:
		return consts.StatusUnauthorized
	case apperror.CodeForbidden:
		return consts.StatusForbidden
	case apperror.CodeOrderNotFound, apperror.CodeRefundNotFound, apperror.CodeNotFound:
		return consts.StatusNotFound
	case apperror.CodeOrderStatusInvalid, apperror.CodeRefundNotAllowed, apperror.CodeRefundStatusInvalid, apperror.CodeConflict:
		return consts.StatusConflict
	case apperror.CodeStockInsufficient:
		return consts.StatusConflict
	case apperror.CodeStockNotFound, apperror.CodeProductNotFound:
		return consts.StatusNotFound
	case apperror.CodeStockReserveFailed, apperror.CodeStockReconcileFailed:
		return consts.StatusServiceUnavailable
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.InvalidArgument:
			return consts.StatusBadRequest
		case codes.FailedPrecondition, codes.Aborted:
			return consts.StatusConflict
		case codes.NotFound:
			return consts.StatusNotFound
		case codes.Unauthenticated:
			return consts.StatusUnauthorized
		case codes.PermissionDenied:
			return consts.StatusForbidden
		case codes.Unavailable:
			return consts.StatusServiceUnavailable
		}
	}
	return consts.StatusBadGateway
}

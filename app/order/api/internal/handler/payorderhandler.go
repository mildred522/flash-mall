package handler

import (
	"net/http"

	"flash-mall/app/order/api/internal/logic"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func PayOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PayOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.PaymentOrderId != "" || req.OutTradeNo != "" {
			resp, err := svcCtx.OrderRpc.MarkOrderPaid(r.Context(), &orderclient.MarkOrderPaidReq{
				OrderId:        req.OrderId,
				PaymentOrderId: req.PaymentOrderId,
				OutTradeNo:     req.OutTradeNo,
				CallbackBody:   `{"trade_status":"SUCCESS","source":"mock"}`,
			})
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			httpx.OkJsonCtx(r.Context(), w, resp)
			return
		}

		if svcCtx.SessionValidator != nil {
			if err := svcCtx.SessionValidator.Validate(r.Context(), r.Header.Get("Authorization")); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		identity, err := extractAuthIdentity(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewPayOrderLogic(r.Context(), svcCtx)
		resp, err := l.PayOrder(&req, identity.UserID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

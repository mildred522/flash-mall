package handler

import (
	"net/http"

	"flash-mall/app/entry/api/internal/logic"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func OrderStatusPollHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderStatusPollReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewOrderStatusPollLogic(r.Context(), svcCtx)
		resp, err := l.OrderStatusPoll(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

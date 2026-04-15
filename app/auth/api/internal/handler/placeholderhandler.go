package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NotImplementedHandler(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unimplemented, action+" not implemented yet"))
	}
}

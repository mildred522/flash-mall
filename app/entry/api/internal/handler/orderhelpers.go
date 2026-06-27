package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/entry/api/internal/svc"
	orderrpc "flash-mall/app/order/rpc/orderclient"
	"flash-mall/app/product/rpc/productclient"
)

func orderStatusText(statusCode int64) string {
	return orderstatus.Text(statusCode)
}

func paymentStatusText(statusCode int64) string {
	return paymentstatus.Text(statusCode)
}

func parsePositiveInt64Query(r *http.Request, name string) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func writeBadRequest(w http.ResponseWriter, message string) {
	writeJSONError(w, http.StatusBadRequest, message)
}

func writeNotFound(w http.ResponseWriter, message string) {
	writeJSONError(w, http.StatusNotFound, message)
}

func writeConflict(w http.ResponseWriter, message string) {
	writeJSONError(w, http.StatusConflict, message)
}

func invalidateAdminCatalogCache(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.CatalogCache != nil {
		svcCtx.CatalogCache.Invalidate(ctx)
	}
}

func compensateClosedOrderInventory(ctx context.Context, svcCtx *svc.ServiceContext, productID, amount int64, orderID string) error {
	if _, err := svcCtx.ProductRpc.RevertStock(ctx, &productclient.RevertStockReq{
		Id:      productID,
		Num:     amount,
		OrderId: orderID,
	}); err != nil {
		return err
	}
	if _, err := svcCtx.OrderRpc.PreDeductRollback(ctx, &orderrpc.PreDeductReq{
		ProductId: productID,
		Amount:    amount,
		OrderId:   orderID,
	}); err != nil {
		return err
	}
	invalidateAdminCatalogCache(ctx, svcCtx)
	return nil
}

package logic

import (
	"context"
	"testing"

	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLegacyStockMutationsAreDisabledByDefault(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "deduct", call: func() error {
			_, err := NewDeductLogic(context.Background(), svcCtx).Deduct(&product.DeductReq{})
			return err
		}},
		{name: "rollback", call: func() error {
			_, err := NewDeductRollbackLogic(context.Background(), svcCtx).DeductRollback(&product.DeductReq{})
			return err
		}},
		{name: "revert", call: func() error {
			_, err := NewRevertStockLogic(context.Background(), svcCtx).RevertStock(&product.RevertStockReq{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := status.Code(tt.call()); code != codes.FailedPrecondition {
				t.Fatalf("status code=%s, want %s", code, codes.FailedPrecondition)
			}
		})
	}
}

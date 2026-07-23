package orderquery

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/common/refundstatus"
)

type BackofficeService struct{ repository BackofficeRepository }

func NewBackofficeService(repository BackofficeRepository) *BackofficeService {
	return &BackofficeService{repository: repository}
}

func (s *BackofficeService) ListAdminOrders(ctx context.Context, query AdminListQuery) (BackofficeOrderList, error) {
	items, total, err := s.repository.ListOrders(ctx, query)
	decorateOrderItems(items)
	return BackofficeOrderList{Items: items, Total: total}, err
}

func (s *BackofficeService) AdminDetail(ctx context.Context, orderID string) (Detail, error) {
	detail, found, err := s.repository.DetailByID(ctx, orderID)
	if err != nil {
		return Detail{}, err
	}
	if !found {
		return Detail{}, apperror.New(apperror.CodeOrderNotFound, "order not found")
	}
	detail.StatusText = orderstatus.Text(detail.Status)
	detail.PaymentStatusText = paymentstatus.Text(detail.PaymentStatus)
	return detail, nil
}

func (s *BackofficeService) AdminStatusLogs(ctx context.Context, orderID string) (StatusLogList, error) {
	items, err := s.repository.StatusLogs(ctx, orderID)
	for index := range items {
		items[index].FromStatusText = orderstatus.Text(items[index].FromStatus)
		items[index].ToStatusText = orderstatus.Text(items[index].ToStatus)
	}
	return StatusLogList{Items: items}, err
}

func (s *BackofficeService) ListMerchantOrders(ctx context.Context, query MerchantListQuery) (BackofficeOrderList, error) {
	items, total, err := s.repository.ListMerchantOrders(ctx, query)
	decorateOrderItems(items)
	return BackofficeOrderList{Items: items, Total: total}, err
}

func (s *BackofficeService) ListMerchantRefunds(ctx context.Context, query RefundListQuery) (RefundList, error) {
	items, total, err := s.repository.ListMerchantRefunds(ctx, query)
	for index := range items {
		items[index].StatusText = refundstatus.Text(items[index].Status)
	}
	return RefundList{Items: items, Total: total}, err
}

func (s *BackofficeService) ListAdminRefunds(ctx context.Context, query RefundListQuery) (RefundList, error) {
	items, total, err := s.repository.ListAdminRefunds(ctx, query)
	for index := range items {
		items[index].StatusText = refundstatus.Text(items[index].Status)
	}
	return RefundList{Items: items, Total: total}, err
}

func decorateOrderItems(items []BackofficeOrderItem) {
	for index := range items {
		items[index].StatusText = orderstatus.Text(items[index].Status)
	}
}

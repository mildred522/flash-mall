import { authed } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminProductCreateReq,
  AdminProductCreateResp,
  AdminProductDetailResp,
  AdminProductListResp,
  AdminProductStockAdjustReq,
  AdminProductStockAdjustResp,
  AdminProductUpdateReq,
  AdminSupplierListResp,
  ApiResponse,
} from '@flash-mall/shared';

export type ProductListParams = Record<string, unknown> & {
  current?: number;
  pageSize?: number;
  product_id?: number;
  supplier_id?: number;
  status?: number;
  promotion_status?: number;
  stock_status?: number;
  keyword?: string;
  initialProductId?: number;
  initialSupplierId?: number;
  initialStockStatus?: number;
};

export async function loadActiveSuppliers() {
  return authed<AdminSupplierListResp>('/api/admin/suppliers?status=1&page=1&page_size=100');
}

export async function loadProductDetail(productId: number) {
  return authed<AdminProductDetailResp>(
    `/api/admin/products/detail?product_id=${encodeURIComponent(String(productId))}`,
  );
}

export async function createProduct(body: AdminProductCreateReq) {
  return authed<AdminProductCreateResp>('/api/admin/products/create', { method: 'POST', jsonBody: body });
}

export async function updateProduct(body: AdminProductUpdateReq) {
  return authed<AdminMutationResp>('/api/admin/products/update', { method: 'POST', jsonBody: body });
}

export async function adjustProductStock(body: AdminProductStockAdjustReq) {
  return authed<AdminProductStockAdjustResp>('/api/admin/products/stock-adjust', { method: 'POST', jsonBody: body });
}

export async function loadProducts(params: ProductListParams) {
  const query = new URLSearchParams();
  query.set('page', String(params.current || 1));
  query.set('page_size', String(params.pageSize || 20));
  const productId = params.product_id || params.initialProductId;
  if (productId) query.set('product_id', String(productId));
  const supplierId = params.supplier_id || params.initialSupplierId;
  if (supplierId) query.set('supplier_id', String(supplierId));
  setOptionalFilter(query, 'status', params.status);
  setOptionalFilter(query, 'promotion_status', params.promotion_status);
  setOptionalFilter(query, 'stock_status', params.stock_status ?? params.initialStockStatus);
  if (params.keyword) query.set('keyword', String(params.keyword));

  const response: ApiResponse<AdminProductListResp> = await authed(`/api/admin/products?${query}`);
  return {
    data: response.ok ? response.data.items || [] : [],
    total: response.ok ? response.data.total : 0,
    success: response.ok,
  };
}

function setOptionalFilter(query: URLSearchParams, key: string, value: unknown) {
  if (value !== undefined && String(value) !== '-1') query.set(key, String(value));
}

import { useEffect, useRef, useState } from 'react';
import { Form, message } from 'antd';
import type { ActionType } from '@ant-design/pro-components';
import { uploadProductImage } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminProductCreateReq,
  AdminProductItem,
  AdminProductStockAdjustReq,
  AdminProductUpdateReq,
  AdminSupplierItem,
} from '@flash-mall/shared';
import {
  adjustProductStock,
  createProduct,
  loadActiveSuppliers,
  loadProductDetail,
  loadProducts,
  updateProduct,
} from './productApi';
import { mutationError, type ProductFormValues, type StockFormValues } from './productModel';

function consumeInitialFilters() {
  const filters = {
    initialProductId: window.__flashAdminProductProductId || 0,
    initialSupplierId: window.__flashAdminProductSupplierId || 0,
    initialStockStatus: window.__flashAdminProductStockStatus,
  };
  window.__flashAdminProductProductId = 0;
  window.__flashAdminProductSupplierId = 0;
  window.__flashAdminProductStockStatus = undefined;
  return filters;
}

export function useProductManagement() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [productForm] = Form.useForm<ProductFormValues>();
  const [stockForm] = Form.useForm<StockFormValues>();
  const [productModalOpen, setProductModalOpen] = useState(false);
  const [stockModalOpen, setStockModalOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState<AdminProductItem | null>(null);
  const [stockProduct, setStockProduct] = useState<AdminProductItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<AdminProductItem | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [suppliers, setSuppliers] = useState<AdminSupplierItem[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [selectedImageFile, setSelectedImageFile] = useState<File | null>(null);
  const [initialFilters] = useState(consumeInitialFilters);

  const reload = () => actionRef.current?.reload();

  const closeProductModal = () => {
    setProductModalOpen(false);
    setEditingProduct(null);
    productForm.resetFields();
    setSelectedImageFile(null);
  };

  const openDetail = async (productId: number) => {
    if (!productId) return;
    setDetailOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const response = await loadProductDetail(productId);
      const error = mutationError(response.data as AdminMutationResp);
      if (response.ok && !error) setDetail(response.data);
      else {
        message.error(error || '商品详情加载失败');
        setDetailOpen(false);
      }
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    void loadActiveSuppliers().then((response) => {
      if (response.ok) setSuppliers(response.data.items || []);
    });
  }, []);

  useEffect(() => {
    if (initialFilters.initialProductId) void openDetail(initialFilters.initialProductId);
  }, [initialFilters.initialProductId]);

  const openCreate = () => {
    setEditingProduct(null);
    productForm.resetFields();
    productForm.setFieldsValue({
      name: '', image_url: '', origin_price_fen: 0, sale_price_fen: 0,
      supplier_id: suppliers[0]?.supplier_id || 0, stock_available: 0, status: 1,
    });
    setProductModalOpen(true);
    setSelectedImageFile(null);
  };

  const openEdit = (product: AdminProductItem) => {
    setEditingProduct(product);
    productForm.setFieldsValue({
      name: product.name,
      image_url: product.image_url || '',
      origin_price_fen: product.origin_price_fen,
      sale_price_fen: product.sale_price_fen,
      supplier_id: product.supplier_id,
      status: product.status,
    });
    setProductModalOpen(true);
    setSelectedImageFile(null);
  };

  const openStock = (product: AdminProductItem) => {
    setStockProduct(product);
    stockForm.setFieldsValue({ delta: 0, bucket_idx: 0 });
    setStockModalOpen(true);
  };

  const saveProduct = async () => {
    const values = await productForm.validateFields();
    if (values.sale_price_fen > values.origin_price_fen) {
      message.warning('现价不能高于原价');
      return;
    }
    setSubmitting(true);
    try {
      let imageURL = values.image_url?.trim() || '';
      if (selectedImageFile) {
        imageURL = await uploadProductImage(selectedImageFile, '/api/admin/products/image');
        productForm.setFieldValue('image_url', imageURL);
      }
      if (editingProduct) await saveExistingProduct(editingProduct, values, imageURL);
      else await saveNewProduct(values, imageURL);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '商品保存失败');
    } finally {
      setSubmitting(false);
    }
  };

  const saveExistingProduct = async (product: AdminProductItem, values: ProductFormValues, imageURL: string) => {
    const body: AdminProductUpdateReq = {
      product_id: product.product_id,
      name: values.name,
      image_url: imageURL,
      origin_price_fen: values.origin_price_fen,
      sale_price_fen: values.sale_price_fen,
      supplier_id: values.supplier_id,
      status: values.status,
    };
    const response = await updateProduct(body);
    const error = mutationError(response.data);
    if (!response.ok || error) {
      message.error(error || '商品更新失败');
      return;
    }
    message.success('商品已更新');
    closeProductModal();
    if (detail?.product_id === product.product_id) void openDetail(product.product_id);
    reload();
  };

  const saveNewProduct = async (values: ProductFormValues, imageURL: string) => {
    const body: AdminProductCreateReq = {
      name: values.name,
      image_url: imageURL,
      origin_price_fen: values.origin_price_fen,
      sale_price_fen: values.sale_price_fen,
      supplier_id: values.supplier_id,
      stock_available: values.stock_available || 0,
      status: values.status,
    };
    const response = await createProduct(body);
    const error = mutationError(response.data);
    if (!response.ok || error) {
      message.error(error || '商品创建失败');
      return;
    }
    message.success(`商品已创建：${response.data.product_id}`);
    closeProductModal();
    reload();
  };

  const adjustStock = async () => {
    if (!stockProduct) return;
    const values = await stockForm.validateFields();
    const body: AdminProductStockAdjustReq = {
      product_id: stockProduct.product_id,
      delta: values.delta,
      bucket_idx: values.bucket_idx || 0,
    };
    setSubmitting(true);
    try {
      const response = await adjustProductStock(body);
      const error = mutationError(response.data);
      if (!response.ok || error) {
        message.error(error || '库存调整失败');
        return;
      }
      message.success(`库存已调整，当前库存 ${response.data.stock_available}`);
      if (detail?.product_id === stockProduct.product_id) {
        setDetail({ ...detail, stock_available: response.data.stock_available });
      }
      setStockModalOpen(false);
      reload();
    } finally {
      setSubmitting(false);
    }
  };

  const toggleStatus = async (product: AdminProductItem) => {
    const nextStatus = product.status === 1 ? 2 : 1;
    const response = await updateProduct({ product_id: product.product_id, status: nextStatus });
    const error = mutationError(response.data);
    if (!response.ok || error) {
      message.error(error || '状态更新失败');
      return;
    }
    message.success(nextStatus === 1 ? '商品已上架' : '商品已下架');
    if (detail?.product_id === product.product_id) {
      setDetail({ ...detail, status: nextStatus, status_text: nextStatus === 1 ? 'active' : 'inactive' });
    }
    reload();
  };

  const navigateToProduct = (path: string, productId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path, productId } }));
  };

  const openSecurityLogs = (productId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/security', securityKeyword: `product:${productId}` },
    }));
  };

  return {
    actionRef,
    productForm,
    stockForm,
    productModalOpen,
    stockModalOpen,
    editingProduct,
    stockProduct,
    detailOpen,
    detail,
    detailLoading,
    suppliers,
    submitting,
    selectedImageFile,
    initialFilters,
    tableRequest: loadProducts,
    openCreate,
    openEdit,
    openStock,
    openDetail,
    openSecurityLogs,
    navigateToProduct,
    saveProduct,
    adjustStock,
    toggleStatus,
    closeProductModal,
    closeStockModal: () => setStockModalOpen(false),
    closeDetail: () => setDetailOpen(false),
    setSelectedImageFile,
  };
}

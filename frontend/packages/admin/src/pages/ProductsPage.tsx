import { Button } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import type { AdminProductItem } from '@flash-mall/shared';
import ProductEditorModal from '../components/products/ProductEditorModal';
import ProductDetailModal from '../components/products/ProductDetailModal';
import StockAdjustModal from '../components/products/StockAdjustModal';
import { createProductColumns } from '../components/products/productColumns';
import { useProductManagement } from '../components/products/useProductManagement';

export default function ProductsPage() {
  const product = useProductManagement();
  const columns = createProductColumns(product.suppliers, {
    detail: product.openDetail,
    edit: product.openEdit,
    stock: product.openStock,
    security: product.openSecurityLogs,
    navigate: product.navigateToProduct,
    status: product.toggleStatus,
  });

  return (
    <>
      <ProTable<AdminProductItem>
        actionRef={product.actionRef}
        columns={columns}
        rowKey="product_id"
        params={product.initialFilters}
        request={product.tableRequest}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          <Button key="create" type="primary" icon={<PlusOutlined />} onClick={product.openCreate}>
            新增商品
          </Button>,
        ]}
      />

      <ProductEditorModal
        open={product.productModalOpen}
        editingProduct={product.editingProduct}
        form={product.productForm}
        suppliers={product.suppliers}
        selectedImageFile={product.selectedImageFile}
        submitting={product.submitting}
        onCancel={product.closeProductModal}
        onSave={product.saveProduct}
        onImageSelected={product.setSelectedImageFile}
      />

      <StockAdjustModal
        open={product.stockModalOpen}
        product={product.stockProduct}
        form={product.stockForm}
        submitting={product.submitting}
        onCancel={product.closeStockModal}
        onSave={product.adjustStock}
      />

      <ProductDetailModal
        open={product.detailOpen}
        product={product.detail}
        loading={product.detailLoading}
        onClose={product.closeDetail}
        onSecurity={(item) => {
          product.closeDetail();
          product.openSecurityLogs(item.product_id);
        }}
        onStock={product.openStock}
        onStatus={product.toggleStatus}
        onNavigate={(path, item) => {
          product.closeDetail();
          product.navigateToProduct(path, item.product_id);
        }}
      />
    </>
  );
}

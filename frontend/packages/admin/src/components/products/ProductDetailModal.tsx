import { Button, Descriptions, Modal } from 'antd';
import { formatPriceFen } from '@flash-mall/shared';
import type { AdminProductItem } from '@flash-mall/shared';
import { ProductStatusTag } from './productModel';

type Props = {
  open: boolean;
  product: AdminProductItem | null;
  loading: boolean;
  onClose: () => void;
  onSecurity: (product: AdminProductItem) => void;
  onStock: (product: AdminProductItem) => void;
  onStatus: (product: AdminProductItem) => void;
  onNavigate: (path: string, product: AdminProductItem) => void;
};

export default function ProductDetailModal(props: Props) {
  const { open, product, loading, onClose, onSecurity, onStock, onStatus, onNavigate } = props;
  const withProduct = (action: (item: AdminProductItem) => void) => () => {
    if (product) action(product);
  };

  return (
    <Modal title="商品详情" open={open} destroyOnHidden onCancel={onClose} footer={[
      <Button key="close" onClick={onClose}>关闭</Button>,
      <Button key="security" disabled={!product} onClick={withProduct(onSecurity)}>安全日志</Button>,
      <Button key="stock" disabled={!product} onClick={withProduct(onStock)}>调整库存</Button>,
      <Button key="status" danger={product?.status === 1} disabled={!product} onClick={withProduct(onStatus)}>
        {product?.status === 1 ? '下架' : '上架'}
      </Button>,
      <Button key="promotions" disabled={!product} onClick={withProduct((item) => onNavigate('/admin/promotions', item))}>查看促销</Button>,
      <Button key="orders" type="primary" disabled={!product} onClick={withProduct((item) => onNavigate('/admin/orders', item))}>查看订单</Button>,
    ]}>
      {product ? (
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="商品ID">{product.product_id}</Descriptions.Item>
          <Descriptions.Item label="状态"><ProductStatusTag status={product.status} /></Descriptions.Item>
          <Descriptions.Item label="名称" span={2}>{product.name || '-'}</Descriptions.Item>
          <Descriptions.Item label="商品图片" span={2}>
            {product.image_url
              ? <img src={product.image_url} alt={`${product.name} 商品图`} style={{ maxWidth: 240, maxHeight: 160, borderRadius: 8, objectFit: 'contain' }} />
              : '无图'}
          </Descriptions.Item>
          <Descriptions.Item label="供应商" span={2}>
            {product.supplier_name ? `${product.supplier_name} (${product.supplier_id})` : product.supplier_id || '-'}
          </Descriptions.Item>
          <Descriptions.Item label="原价">¥{formatPriceFen(product.origin_price_fen)}</Descriptions.Item>
          <Descriptions.Item label="售价">¥{formatPriceFen(product.sale_price_fen)}</Descriptions.Item>
          <Descriptions.Item label="当前价">¥{formatPriceFen(product.promotion_price_fen > 0 ? product.promotion_price_fen : product.sale_price_fen)}</Descriptions.Item>
          <Descriptions.Item label="库存">{product.stock_available || 0}</Descriptions.Item>
          <Descriptions.Item label="活动类型">{product.promotion_type || '-'}</Descriptions.Item>
          <Descriptions.Item label="活动标签">{product.promotion_tag || '-'}</Descriptions.Item>
        </Descriptions>
      ) : loading ? <div>加载中...</div> : <div>暂无商品详情</div>}
    </Modal>
  );
}

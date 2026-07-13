import { PRODUCT_META, formatPriceFen } from '@flash-mall/shared';
import type { ProductCard as ProductCardType } from '@flash-mall/shared';

interface Props {
  product: ProductCardType;
  onBuy: (productId: number) => void;
}

export default function ProductCard({ product, onBuy }: Props) {
  const meta = PRODUCT_META[product.product_id];
  const icon = meta?.icon || '📦';
  const hasDiscount = product.final_price_fen < product.origin_price_fen;

  return (
    <div className="product" onClick={() => onBuy(product.product_id)}>
      <div className="thumb">
        <div className="badge">
          {product.promotion_tag && <span className="pill orange">{product.promotion_tag}</span>}
        </div>
        <span className="thumb-icon">{icon}</span>
      </div>
      <div className="product-info">
        <div className="product-name">{product.name}</div>
        <div className="price-row">
          <span className="price-sale">
            <span className="symbol">¥</span>{formatPriceFen(product.final_price_fen)}
          </span>
          {hasDiscount && (
            <span className="price-origin">¥{formatPriceFen(product.origin_price_fen)}</span>
          )}
        </div>
        <div className="product-meta">
          <span>库存 {product.stock_available}</span>
          <button className="btn-buy" onClick={(e) => { e.stopPropagation(); onBuy(product.product_id); }}>
            立即购买
          </button>
        </div>
      </div>
    </div>
  );
}

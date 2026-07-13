import { useState } from 'react';
import { PRODUCT_META, formatPriceFen, resolveProductImage } from '@flash-mall/shared';
import type { ProductCard as ProductCardType } from '@flash-mall/shared';

interface Props {
  product: ProductCardType;
  onView: (productId: number) => void;
  onStore?: (merchantId: number) => void;
  onBuy: (productId: number) => void;
}

export default function ProductCard({ product, onView, onStore, onBuy }: Props) {
  const meta = PRODUCT_META[product.product_id];
  const icon = meta?.icon || '📦';
  const imageURL = resolveProductImage(product.product_id, product.image_url);
  const [imageFailed, setImageFailed] = useState(false);
  const hasDiscount = product.final_price_fen < product.origin_price_fen;
  const canOpenStore = Boolean(product.merchant_id && onStore);
  const soldOut = product.stock_available <= 0;

  return (
    <article className="product">
      <button
        type="button"
        className="product-view"
        aria-label={`查看${product.name}`}
        onClick={() => onView(product.product_id)}
      >
        <div className="thumb">
          <div className="badge">
            {product.promotion_tag && <span className="pill orange">{product.promotion_tag}</span>}
          </div>
          {imageURL && !imageFailed ? (
            <img className="thumb-img" src={imageURL} alt={product.name || '商品图片'} loading="lazy" onError={() => setImageFailed(true)} />
          ) : (
            <span className="thumb-icon">{icon}</span>
          )}
        </div>
        <div className="product-info">
          <div className="product-name">{product.name}</div>
          <div className="price-row">
            <span className="price-sale">
              <span className="symbol">¥</span>{formatPriceFen(product.final_price_fen)}
            </span>
            {hasDiscount && <span className="price-origin">¥{formatPriceFen(product.origin_price_fen)}</span>}
          </div>
        </div>
      </button>

      {canOpenStore && (
        <button
          type="button"
          className="product-merchant"
          aria-label={`进入${product.merchant_name || '商家店铺'}`}
          onClick={() => onStore?.(product.merchant_id!)}
        >
          <span className="product-merchant-logo">
            {product.merchant_logo ? <img src={product.merchant_logo} alt="" /> : '店'}
          </span>
          <span>{product.merchant_name || '认证商家'}</span>
          <em>逛店 →</em>
        </button>
      )}

      <div className="product-meta">
        <span>{soldOut ? '库存已售罄' : `库存 ${product.stock_available}`}</span>
        <button
          type="button"
          className="btn-buy"
          disabled={soldOut}
          onClick={() => onBuy(product.product_id)}
        >
          {soldOut ? '已售罄' : '立即购买'}
        </button>
      </div>
    </article>
  );
}

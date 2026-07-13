import { useEffect, useState } from 'react';
import {
  PRODUCT_META,
  api,
  formatPriceFen,
  resolveProductImage,
} from '@flash-mall/shared';
import type { ProductDetailResp } from '@flash-mall/shared';
import { navigateShop } from '../App';

interface Props {
  productId: number;
  onBuy: (productId: number) => void;
}

export default function ProductDetailPage({ productId, onBuy }: Props) {
  const [detail, setDetail] = useState<ProductDetailResp | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [imageFailed, setImageFailed] = useState(false);

  useEffect(() => {
    setLoading(true);
    setFailed(false);
    setImageFailed(false);
    api<ProductDetailResp>(`/api/shop/products/detail?product_id=${productId}`)
      .then((res) => {
        if (res.ok) setDetail(res.data);
        else setFailed(true);
      })
      .catch(() => setFailed(true))
      .finally(() => setLoading(false));
  }, [productId]);

  if (loading) return <div className="container page-state">正在加载商品...</div>;
  if (failed || !detail?.item) {
    return (
      <div className="container page-state">
        <h1>商品暂时无法查看</h1>
        <p>商品可能已经下架，或所属店铺暂停营业。</p>
        <button className="btn-primary" onClick={() => navigateShop('/shop')}>返回首页</button>
      </div>
    );
  }

  const { item, store_products: storeProducts } = detail;
  const imageURL = resolveProductImage(item.product_id, item.image_url);
  const fallbackIcon = PRODUCT_META[item.product_id]?.icon || '📦';
  const hasDiscount = item.final_price_fen < item.origin_price_fen;

  return (
    <div className="container detail-page">
      <button className="text-link detail-back" onClick={() => navigateShop('/shop')}>← 返回首页</button>
      <section className="detail-layout">
        <div className="detail-media">
          {imageURL && !imageFailed ? (
            <img src={imageURL} alt={item.name} onError={() => setImageFailed(true)} />
          ) : (
            <span aria-label="商品图片占位">{fallbackIcon}</span>
          )}
        </div>
        <div className="detail-copy">
          {item.promotion_tag && <span className="pill orange">{item.promotion_tag}</span>}
          <h1>{item.name}</h1>
          <div className="detail-price">
            <span><small>¥</small>{formatPriceFen(item.final_price_fen)}</span>
            {hasDiscount && <del>¥{formatPriceFen(item.origin_price_fen)}</del>}
          </div>
          <p className="detail-stock">现货库存 {item.stock_available} 件 · 结算前将再次确认库存</p>
          <button
            className="btn-primary detail-buy"
            disabled={item.stock_available <= 0}
            onClick={() => onBuy(item.product_id)}
          >
            {item.stock_available > 0 ? '立即购买' : '暂时售罄'}
          </button>

          {item.merchant_id && (
            <div className="merchant-brief">
              <div className="merchant-avatar">
                {item.merchant_logo ? <img src={item.merchant_logo} alt="店铺标志" /> : '店'}
              </div>
              <div>
                <span>本商品由</span>
                <strong>{item.merchant_name || '认证商家'}</strong>
              </div>
              <button className="btn-secondary" onClick={() => navigateShop(`/store/${item.merchant_id}`)}>
                进入店铺 →
              </button>
            </div>
          )}
        </div>
      </section>

      {storeProducts.length > 0 && (
        <section className="section detail-related">
          <div className="section-title">
            <div>
              <span className="eyebrow">MORE FROM THIS STORE</span>
              <h2>同店好物</h2>
            </div>
          </div>
          <div className="related-strip">
            {storeProducts.map((product) => (
              <button
                key={product.product_id}
                className="related-product"
                onClick={() => navigateShop(`/product/${product.product_id}`)}
              >
                <span>{PRODUCT_META[product.product_id]?.icon || '📦'}</span>
                <strong>{product.name}</strong>
                <em>¥{formatPriceFen(product.final_price_fen)}</em>
              </button>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

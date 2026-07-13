import { useEffect, useState } from 'react';
import {
  PRODUCT_META,
  api,
  formatPriceFen,
  resolveProductImage,
} from '@flash-mall/shared';
import type { ProductCard, PublicStoreDetail, StoreProductsResp } from '@flash-mall/shared';
import { navigateShop } from '../navigation';

interface Props {
  merchantId: number;
  onBuy: (productId: number) => void;
}

const PAGE_SIZE = 12;

export default function StorePage({ merchantId, onBuy }: Props) {
  const [store, setStore] = useState<PublicStoreDetail | null>(null);
  const [products, setProducts] = useState<ProductCard[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setLoading(true);
    setFailed(false);
    Promise.all([
      api<PublicStoreDetail>(`/api/shop/stores/detail?merchant_id=${merchantId}`),
      api<StoreProductsResp>(`/api/shop/stores/products?merchant_id=${merchantId}&page=${page}&page_size=${PAGE_SIZE}`),
    ]).then(([storeRes, productsRes]) => {
      if (!storeRes.ok || !productsRes.ok) {
        setFailed(true);
        return;
      }
      setStore(storeRes.data);
      setProducts(productsRes.data.items || []);
      setTotal(productsRes.data.total || 0);
    }).catch(() => setFailed(true)).finally(() => setLoading(false));
  }, [merchantId, page]);

  if (loading) return <div className="container page-state">正在走进店铺...</div>;
  if (failed || !store) {
    return (
      <div className="container page-state">
        <h1>店铺暂时无法访问</h1>
        <p>店铺可能已暂停营业。</p>
        <button className="btn-primary" onClick={() => navigateShop('/shop')}>返回首页</button>
      </div>
    );
  }

  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="container store-page">
      <button className="text-link detail-back" onClick={() => navigateShop('/shop')}>← 返回首页</button>
      <section
        className={`store-hero${store.banner_url ? ' has-banner' : ''}`}
        style={store.banner_url ? { backgroundImage: `linear-gradient(90deg, rgba(28,22,18,.86), rgba(28,22,18,.28)), url(${store.banner_url})` } : undefined}
      >
        <div className="store-logo">
          {store.logo_url ? <img src={store.logo_url} alt={`${store.merchant_name}标志`} /> : store.merchant_name.slice(0, 1)}
        </div>
        <div className="store-identity">
          <span className="eyebrow">MERCHANT STOREFRONT</span>
          <h1>{store.merchant_name}</h1>
          <p>{store.description || '店主正在认真准备店铺介绍。'}</p>
          <span>{store.product_count} 件在售商品</span>
        </div>
      </section>

      <section className="section store-products">
        <div className="section-title">
          <div>
            <span className="eyebrow">CURATED BY THE MERCHANT</span>
            <h2>店内商品</h2>
          </div>
          <span className="pill outline">共 {total} 件</span>
        </div>
        {products.length === 0 ? (
          <div className="store-empty">店主还没有上架商品，稍后再来看看。</div>
        ) : (
          <div className="store-product-grid">
            {products.map((product) => {
              const imageURL = resolveProductImage(product.product_id, product.image_url);
              return (
                <article className="store-product" key={product.product_id}>
                  <button
                    className="store-product-main"
                    aria-label={`查看${product.name}`}
                    onClick={() => navigateShop(`/product/${product.product_id}`)}
                  >
                    <div className="store-product-image">
                      {imageURL ? <img src={imageURL} alt={product.name} /> : <span>{PRODUCT_META[product.product_id]?.icon || '📦'}</span>}
                    </div>
                    <h3>{product.name}</h3>
                    <strong>¥{formatPriceFen(product.final_price_fen)}</strong>
                  </button>
                  <button className="store-product-buy" disabled={product.stock_available <= 0} onClick={() => onBuy(product.product_id)}>
                    {product.stock_available > 0 ? '立即购买' : '已售罄'}
                  </button>
                </article>
              );
            })}
          </div>
        )}
        {pages > 1 && (
          <div className="store-pagination">
            <button disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>上一页</button>
            <span>{page} / {pages}</span>
            <button disabled={page >= pages} onClick={() => setPage((value) => value + 1)}>下一页</button>
          </div>
        )}
      </section>
    </div>
  );
}

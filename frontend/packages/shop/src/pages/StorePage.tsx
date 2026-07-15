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

function StoreProductVisual({ product }: { product: ProductCard }) {
  const [failed, setFailed] = useState(false);
  const imageURL = resolveProductImage(product.product_id, product.image_url);

  if (!imageURL || failed) {
    return <span aria-hidden="true">{PRODUCT_META[product.product_id]?.icon || '精选'}</span>;
  }
  return <img src={imageURL} alt={product.name} loading="lazy" onError={() => setFailed(true)} />;
}

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
    <div className="store-page">
      <div className="container">
        <button className="text-link detail-back" onClick={() => navigateShop('/shop')}>← 返回商城橱窗</button>
      </div>
      <section
        className={`store-hero${store.banner_url ? ' has-banner' : ''}`}
        style={store.banner_url ? { backgroundImage: `url(${store.banner_url})` } : undefined}
      >
        <div className="store-hero-shade" />
        <div className="container store-hero-content">
          <div className="store-logo">
            <span aria-hidden="true">{store.merchant_name.slice(0, 1)}</span>
            {store.logo_url && <img src={store.logo_url} alt={`${store.merchant_name}标志`} onError={(event) => { event.currentTarget.style.display = 'none'; }} />}
          </div>
          <div className="store-identity">
            <span className="eyebrow">FLASH MALL · INDEPENDENT STORE</span>
            <h1>{store.merchant_name}</h1>
            <p>{store.description || '店主正在认真准备店铺介绍。'}</p>
            <div className="store-facts">
              <span>{store.product_count} 件在售</span>
              <span>商家自营</span>
              <span>平台库存校验</span>
            </div>
            <button className="store-scroll-cta" onClick={() => document.querySelector('#store-products')?.scrollIntoView({ behavior: 'smooth' })}>
              浏览店主选品 <span>↓</span>
            </button>
          </div>
        </div>
      </section>

      <section id="store-products" className="container section store-products">
        <div className="section-title">
          <div>
            <span className="eyebrow">CURATED BY THE MERCHANT</span>
            <h2>店主选品</h2>
          </div>
          <span className="pill outline">共 {total} 件</span>
        </div>
        {products.length === 0 ? (
          <div className="store-empty">店主还没有上架商品，稍后再来看看。</div>
        ) : (
          <div className="store-product-grid">
            {products.map((product, index) => (
                <article className="store-product" key={product.product_id} style={{ animationDelay: `${index * 70}ms` }}>
                  <span className="store-product-index">{String(index + 1).padStart(2, '0')}</span>
                  <button
                    className="store-product-main"
                    aria-label={`查看${product.name}`}
                    onClick={() => navigateShop(`/product/${product.product_id}`)}
                  >
                    <div className="store-product-image">
                      <StoreProductVisual product={product} />
                    </div>
                    <div className="store-product-copy">
                      <span>店主上架 · 库存 {product.stock_available}</span>
                      <h3>{product.name}</h3>
                      <strong>¥{formatPriceFen(product.final_price_fen)}</strong>
                    </div>
                  </button>
                  <button className="store-product-buy" disabled={product.stock_available <= 0} onClick={() => onBuy(product.product_id)}>
                    {product.stock_available > 0 ? '立即购买' : '已售罄'}
                  </button>
                </article>
            ))}
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

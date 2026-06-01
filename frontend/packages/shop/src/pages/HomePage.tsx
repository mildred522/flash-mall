import { useState, useEffect } from 'react';
import { api, authed } from '@flash-mall/shared';
import type { ProductCard as ProductCardType, CatalogResp, CreateOrderResp } from '@flash-mall/shared';
import ProductCard from '../components/ProductCard';
import { useAuth } from '../contexts/AuthContext';

interface Props {
  onLogin: () => void;
  onOrderCreated: (orderId: string, requestId: string) => void;
}

export default function HomePage({ onLogin, onOrderCreated }: Props) {
  const { token } = useAuth();
  const [products, setProducts] = useState<ProductCardType[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api<CatalogResp>('/api/shop/catalog').then((res) => {
      if (res.ok) setProducts(res.data.items || []);
      setLoading(false);
    });
  }, []);

  const handleBuy = async (productId: number) => {
    if (!token) {
      onLogin();
      return;
    }
    const requestId = crypto.randomUUID();
    const res = await authed<CreateOrderResp>('/api/order/create', {
      method: 'POST',
      jsonBody: { request_id: requestId, user_id: 0, product_id: productId, amount: 1 },
    });
    if (res.ok) {
      onOrderCreated(res.data.order_id, requestId);
    } else {
      alert(`下单失败: ${JSON.stringify(res.data)}`);
    }
  };

  return (
    <div className="container">
      <section className="hero">
        <div className="hero-copy">
          <span className="pill">限时秒杀</span>
          <h1>品质好物 限时特惠</h1>
          <p>精选好物，品质保证，限时折扣，先到先得！</p>
        </div>
        <div className="hero-side">
          <div className="card">
            <h3>新人专享</h3>
            <strong>注册即享首单优惠</strong>
            <p>新用户注册即可获得专属折扣</p>
          </div>
          <div className="card">
            <h3>品质保证</h3>
            <strong>100% 正品保障</strong>
            <p>所有商品均为正品，假一赔十</p>
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section-title">
          <h2>热销商品</h2>
          <span className="pill outline">{products.length} 件商品</span>
        </div>
        {loading ? (
          <p style={{ textAlign: 'center', padding: 40 }}>加载中...</p>
        ) : (
          <div className="product-grid">
            {products.map((p) => (
              <ProductCard key={p.product_id} product={p} onBuy={handleBuy} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

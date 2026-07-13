import { useEffect, useState } from 'react';
import { authed } from '@flash-mall/shared';
import type { CreateOrderResp } from '@flash-mall/shared';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import Header from './components/Header';
import AuthModal from './components/AuthModal';
import HomePage from './pages/HomePage';
import OrdersPage from './pages/OrdersPage';
import PaymentPage from './pages/PaymentPage';
import ProductDetailPage from './pages/ProductDetailPage';
import StorePage from './pages/StorePage';
import './styles/shop.css';

export type ShopRoute =
  | { page: 'shop' }
  | { page: 'orders' }
  | { page: 'product'; productId: number }
  | { page: 'store'; merchantId: number };

export function parseShopRoute(pathname: string): ShopRoute {
  if (pathname === '/' || pathname === '/shop') return { page: 'shop' };
  if (pathname === '/orders') return { page: 'orders' };

  const productMatch = pathname.match(/^\/product\/(\d+)\/?$/);
  if (productMatch) return { page: 'product', productId: Number(productMatch[1]) };

  const storeMatch = pathname.match(/^\/store\/(\d+)\/?$/);
  if (storeMatch) return { page: 'store', merchantId: Number(storeMatch[1]) };

  return { page: 'shop' };
}

export function navigateShop(path: string) {
  if (window.location.pathname !== path) window.history.pushState({}, '', path);
  window.dispatchEvent(new Event('flash-shop:navigate'));
}

function AppInner() {
  const { user, token } = useAuth();
  const [route, setRoute] = useState<ShopRoute>(() => parseShopRoute(window.location.pathname));
  const [authOpen, setAuthOpen] = useState(false);
  const [pendingOrder, setPendingOrder] = useState<{ orderId: string; requestId: string } | null>(null);

  useEffect(() => {
    const syncRoute = () => setRoute(parseShopRoute(window.location.pathname));
    window.addEventListener('popstate', syncRoute);
    window.addEventListener('flash-shop:navigate', syncRoute);
    return () => {
      window.removeEventListener('popstate', syncRoute);
      window.removeEventListener('flash-shop:navigate', syncRoute);
    };
  }, []);

  const showOrders = () => {
    if (!user) {
      setAuthOpen(true);
      return;
    }
    navigateShop('/orders');
  };

  const orderCreated = (orderId: string, requestId: string) => {
    setPendingOrder({ orderId, requestId });
    navigateShop('/orders');
  };

  const buyProduct = async (productId: number) => {
    if (!token) {
      setAuthOpen(true);
      return;
    }
    const requestId = crypto.randomUUID();
    const res = await authed<CreateOrderResp>('/api/order/create', {
      method: 'POST',
      jsonBody: { request_id: requestId, user_id: 0, product_id: productId, amount: 1 },
    });
    if (res.ok) {
      orderCreated(res.data.order_id, requestId);
      return;
    }
    alert(`下单失败: ${JSON.stringify(res.data)}`);
  };

  let content;
  switch (route.page) {
    case 'orders':
      content = <OrdersPage pendingOrder={pendingOrder} onPendingHandled={() => setPendingOrder(null)} />;
      break;
    case 'product':
      content = <ProductDetailPage productId={route.productId} onBuy={buyProduct} />;
      break;
    case 'store':
      content = <StorePage merchantId={route.merchantId} onBuy={buyProduct} />;
      break;
    default:
      content = (
        <HomePage
          onLogin={() => setAuthOpen(true)}
          onOrderCreated={orderCreated}
        />
      );
  }

  return (
    <>
      <Header
        onShowShop={() => navigateShop('/shop')}
        onShowOrders={showOrders}
        onLogin={() => setAuthOpen(true)}
      />
      <main className="shop-page-transition" key={`${route.page}-${'productId' in route ? route.productId : 'merchantId' in route ? route.merchantId : ''}`}>
        {content}
      </main>
      <AuthModal open={authOpen} onClose={() => setAuthOpen(false)} />
    </>
  );
}

export default function App() {
  if (window.location.pathname === '/pay') return <PaymentPage />;
  return (
    <AuthProvider>
      <AppInner />
    </AuthProvider>
  );
}

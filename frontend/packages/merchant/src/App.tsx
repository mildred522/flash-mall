import { useEffect, useState, type ComponentType } from 'react';
import { ProLayout } from '@ant-design/pro-components';
import { DashboardOutlined, OrderedListOutlined, ShoppingOutlined, SwapOutlined, UndoOutlined } from '@ant-design/icons';
import MerchantGuard from './components/MerchantGuard';
import DashboardPage from './pages/DashboardPage';
import InventoryPage from './pages/InventoryPage';
import OrdersPage from './pages/OrdersPage';
import ProductsPage from './pages/ProductsPage';
import RefundsPage from './pages/RefundsPage';

const routeMap: Record<string, ComponentType> = {
  '/merchant': DashboardPage,
  '/merchant/products': ProductsPage,
  '/merchant/inventory': InventoryPage,
  '/merchant/orders': OrdersPage,
  '/merchant/refunds': RefundsPage,
};

const menuRoutes = {
  routes: [
    { path: '/merchant', name: '数据概览', icon: <DashboardOutlined /> },
    { path: '/merchant/products', name: '商品管理', icon: <ShoppingOutlined /> },
    { path: '/merchant/inventory', name: '库存流水', icon: <SwapOutlined /> },
    { path: '/merchant/orders', name: '订单发货', icon: <OrderedListOutlined /> },
    { path: '/merchant/refunds', name: '退款查看', icon: <UndoOutlined /> },
  ],
};

export default function App() {
  const initialPath = routeMap[window.location.pathname] ? window.location.pathname : '/merchant';
  const [pathname, setPathname] = useState(initialPath);
  const Page = routeMap[pathname] || DashboardPage;

  const navigate = (path: string) => {
    const next = routeMap[path] ? path : '/merchant';
    window.history.pushState({}, '', next);
    setPathname(next);
  };

  useEffect(() => {
    const onNavigate = (event: Event) => navigate((event as CustomEvent<{ path?: string }>).detail?.path || '/merchant');
    const onPopState = () => setPathname(routeMap[window.location.pathname] ? window.location.pathname : '/merchant');
    window.addEventListener('flash-merchant:navigate', onNavigate);
    window.addEventListener('popstate', onPopState);
    return () => {
      window.removeEventListener('flash-merchant:navigate', onNavigate);
      window.removeEventListener('popstate', onPopState);
    };
  }, []);

  return (
    <MerchantGuard>
      <ProLayout
        title="Flash Mall Merchant"
        logo={<span style={{ fontSize: 20, fontWeight: 800 }}>M</span>}
        route={menuRoutes}
        location={{ pathname }}
        menuItemRender={(item, dom) => <a onClick={() => navigate(item.path || '/merchant')}>{dom}</a>}
        fixSiderbar
        layout="mix"
      >
        <Page />
      </ProLayout>
    </MerchantGuard>
  );
}

import { useState } from 'react';
import { ProLayout } from '@ant-design/pro-components';
import {
  DashboardOutlined,
  OrderedListOutlined,
  ShoppingOutlined,
  UserOutlined,
} from '@ant-design/icons';
import AdminGuard from './components/AdminGuard';
import DashboardPage from './pages/DashboardPage';
import OrdersPage from './pages/OrdersPage';
import ProductsPage from './pages/ProductsPage';
import UsersPage from './pages/UsersPage';

const routeMap: Record<string, () => JSX.Element> = {
  '/admin': DashboardPage,
  '/admin/orders': OrdersPage,
  '/admin/products': ProductsPage,
  '/admin/users': UsersPage,
};

const menuRoutes = {
  routes: [
    { path: '/admin', name: '数据概览', icon: <DashboardOutlined /> },
    { path: '/admin/orders', name: '订单管理', icon: <OrderedListOutlined /> },
    { path: '/admin/products', name: '商品管理', icon: <ShoppingOutlined /> },
    { path: '/admin/users', name: '用户管理', icon: <UserOutlined /> },
  ],
};

export default function App() {
  const [pathname, setPathname] = useState('/admin');
  const PageComponent = routeMap[pathname] || DashboardPage;

  return (
    <AdminGuard>
      <ProLayout
        title="Flash Mall Admin"
        logo={<span style={{ fontSize: 20, fontWeight: 800 }}>F</span>}
        route={menuRoutes}
        location={{ pathname }}
        menuItemRender={(item, dom) => (
          <a onClick={() => setPathname(item.path || '/admin')}>{dom}</a>
        )}
        fixSiderbar
        layout="mix"
      >
        <PageComponent />
      </ProLayout>
    </AdminGuard>
  );
}

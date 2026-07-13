import { useEffect, useState } from 'react';
import { ProLayout } from '@ant-design/pro-components';
import {
  DashboardOutlined,
  OrderedListOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  ShoppingOutlined,
  TagsOutlined,
  UserOutlined,
} from '@ant-design/icons';
import AdminGuard from './components/AdminGuard';
import DashboardPage from './pages/DashboardPage';
import OrdersPage from './pages/OrdersPage';
import ProductsPage from './pages/ProductsPage';
import PromotionsPage from './pages/PromotionsPage';
import SecurityEventsPage from './pages/SecurityEventsPage';
import SuppliersPage from './pages/SuppliersPage';
import UsersPage from './pages/UsersPage';

declare global {
  interface Window {
    __flashAdminPromotionProductId?: number;
    __flashAdminPromotionPromotionId?: number;
    __flashAdminPromotionEffectStatus?: string;
    __flashAdminOrderOrderId?: string;
    __flashAdminOrderProductId?: number;
    __flashAdminOrderUserId?: number;
    __flashAdminOrderStatus?: number;
    __flashAdminUserUserId?: number;
    __flashAdminSecurityUserId?: number;
    __flashAdminSecurityKeyword?: string;
    __flashAdminProductProductId?: number;
    __flashAdminProductSupplierId?: number;
    __flashAdminProductStockStatus?: number;
    __flashAdminSupplierSupplierId?: number;
  }
}

const routeMap: Record<string, () => JSX.Element> = {
  '/admin': DashboardPage,
  '/admin/orders': OrdersPage,
  '/admin/products': ProductsPage,
  '/admin/suppliers': SuppliersPage,
  '/admin/promotions': PromotionsPage,
  '/admin/users': UsersPage,
  '/admin/security': SecurityEventsPage,
};

const menuRoutes = {
  routes: [
    { path: '/admin', name: '数据概览', icon: <DashboardOutlined /> },
    { path: '/admin/orders', name: '订单管理', icon: <OrderedListOutlined /> },
    { path: '/admin/products', name: '商品管理', icon: <ShoppingOutlined /> },
    { path: '/admin/suppliers', name: '供应商管理', icon: <ShopOutlined /> },
    { path: '/admin/promotions', name: '促销管理', icon: <TagsOutlined /> },
    { path: '/admin/users', name: '用户管理', icon: <UserOutlined /> },
    { path: '/admin/security', name: '安全日志', icon: <SafetyCertificateOutlined /> },
  ],
};

export default function App() {
  const [pathname, setPathname] = useState('/admin');
  const PageComponent = routeMap[pathname] || DashboardPage;

  useEffect(() => {
    const handleNavigate = (event: Event) => {
      const detail = (event as CustomEvent<{
        path?: string;
        productId?: number;
        supplierId?: number;
        userId?: number;
        orderId?: string;
        promotionId?: number;
        orderStatus?: number;
        stockStatus?: number;
        effectStatus?: string;
        securityKeyword?: string;
      }>).detail;
      if (detail?.productId || detail?.supplierId || detail?.userId || detail?.orderId || detail?.promotionId || detail?.orderStatus !== undefined || detail?.stockStatus !== undefined || detail?.effectStatus || detail?.securityKeyword) {
        if (detail?.path === '/admin/orders') {
          window.__flashAdminOrderOrderId = detail.orderId;
          window.__flashAdminOrderProductId = detail.productId;
          window.__flashAdminOrderUserId = detail.userId;
          window.__flashAdminOrderStatus = detail.orderStatus;
        } else if (detail?.path === '/admin/products') {
          window.__flashAdminProductProductId = detail.productId;
          window.__flashAdminProductSupplierId = detail.supplierId;
          window.__flashAdminProductStockStatus = detail.stockStatus;
        } else if (detail?.path === '/admin/suppliers') {
          window.__flashAdminSupplierSupplierId = detail.supplierId;
        } else if (detail?.path === '/admin/users') {
          window.__flashAdminUserUserId = detail.userId;
        } else if (detail?.path === '/admin/security') {
          window.__flashAdminSecurityUserId = detail.userId;
          window.__flashAdminSecurityKeyword = detail.securityKeyword;
        } else {
          window.__flashAdminPromotionPromotionId = detail.promotionId;
          window.__flashAdminPromotionProductId = detail.productId;
          window.__flashAdminPromotionEffectStatus = detail.effectStatus;
        }
      }
      if (detail?.path) setPathname(detail.path);
    };
    window.addEventListener('flash-admin:navigate', handleNavigate);
    return () => window.removeEventListener('flash-admin:navigate', handleNavigate);
  }, []);

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

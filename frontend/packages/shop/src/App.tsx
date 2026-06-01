import { useState } from 'react';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import Header from './components/Header';
import AuthModal from './components/AuthModal';
import HomePage from './pages/HomePage';
import OrdersPage from './pages/OrdersPage';
import './styles/shop.css';

function AppInner() {
  const { user } = useAuth();
  const [page, setPage] = useState<'shop' | 'orders'>('shop');
  const [authOpen, setAuthOpen] = useState(false);
  const [pendingOrder, setPendingOrder] = useState<{ orderId: string; requestId: string } | null>(null);

  return (
    <>
      <Header
        onShowShop={() => setPage('shop')}
        onShowOrders={() => {
          if (!user) { setAuthOpen(true); return; }
          setPage('orders');
        }}
        onLogin={() => setAuthOpen(true)}
      />
      {page === 'shop' ? (
        <HomePage
          onLogin={() => setAuthOpen(true)}
          onOrderCreated={(orderId, requestId) => {
            setPendingOrder({ orderId, requestId });
            setPage('orders');
          }}
        />
      ) : (
        <OrdersPage pendingOrder={pendingOrder} onPendingHandled={() => setPendingOrder(null)} />
      )}
      <AuthModal open={authOpen} onClose={() => setAuthOpen(false)} />
    </>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <AppInner />
    </AuthProvider>
  );
}

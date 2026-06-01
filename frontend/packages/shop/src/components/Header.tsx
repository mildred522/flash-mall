import { useAuth } from '../contexts/AuthContext';

interface HeaderProps {
  onShowOrders: () => void;
  onShowShop: () => void;
  onLogin: () => void;
}

export default function Header({ onShowOrders, onShowShop, onLogin }: HeaderProps) {
  const { user, logout } = useAuth();

  return (
    <header className="topbar">
      <div className="container topbar-inner">
        <div className="brand">
          <span className="brand-mark">F</span>
          Flash Mall
        </div>
        <nav className="nav">
          <a href="#" onClick={(e) => { e.preventDefault(); onShowShop(); }}>首页</a>
          <a href="#" onClick={(e) => { e.preventDefault(); onShowOrders(); }}>我的订单</a>
        </nav>
        <div className="row">
          {user ? (
            <>
              <span className="chip">{user.display_name}</span>
              <button className="button soft" onClick={logout}>退出</button>
            </>
          ) : (
            <button className="button primary" onClick={onLogin}>登录</button>
          )}
        </div>
      </div>
    </header>
  );
}

import { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { api } from '@flash-mall/shared';

interface Props {
  open: boolean;
  onClose: () => void;
}

export default function AuthModal({ open, onClose }: Props) {
  const { login, register } = useAuth();
  const [tab, setTab] = useState<'login' | 'register'>('login');
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async () => {
    setError('');
    setLoading(true);
    const ok = await login(phone, password);
    setLoading(false);
    if (ok) {
      onClose();
      setPhone('');
      setPassword('');
    } else {
      setError('登录失败，请检查手机号和密码');
    }
  };

  const handleRegister = async () => {
    setError('');
    setLoading(true);
    const ok = await register(phone, password, code);
    setLoading(false);
    if (ok) {
      onClose();
      setPhone('');
      setPassword('');
      setCode('');
    } else {
      setError('注册失败');
    }
  };

  const sendCode = async () => {
    if (!phone) return;
    await api('/api/auth/code/send', {
      method: 'POST',
      jsonBody: { phone, scene: 'register' },
    });
  };

  if (!open) return null;

  return (
    <div className={`auth-modal ${open ? 'open' : ''}`} onClick={onClose}>
      <div className="auth-box" onClick={(e) => e.stopPropagation()}>
        <h2>欢迎来到 Flash Mall</h2>
        <p className="note">登录后即可下单购买</p>

        <div className="tabs">
          <button className={tab === 'login' ? 'active' : ''} onClick={() => { setTab('login'); setError(''); }}>密码登录</button>
          <button className={tab === 'register' ? 'active' : ''} onClick={() => { setTab('register'); setError(''); }}>注册账号</button>
        </div>

        {tab === 'login' ? (
          <div className="panel active">
            <div className="field">
              <label>手机号</label>
              <input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="请输入手机号" />
            </div>
            <div className="field">
              <label>密码</label>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="请输入密码" />
            </div>
            <div className="error">{error}</div>
            <button className="button primary submit" onClick={handleLogin} disabled={loading}>
              {loading ? '登录中...' : '登录'}
            </button>
          </div>
        ) : (
          <div className="panel active">
            <div className="field">
              <label>手机号</label>
              <input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="请输入手机号" />
            </div>
            <div className="field">
              <label>验证码</label>
              <div className="code-row">
                <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="请输入验证码" />
                <button className="button soft" onClick={sendCode}>发送验证码</button>
              </div>
            </div>
            <div className="field">
              <label>密码</label>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="请设置密码" />
            </div>
            <div className="error">{error}</div>
            <button className="button primary submit" onClick={handleRegister} disabled={loading}>
              {loading ? '注册中...' : '注册'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

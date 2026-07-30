import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import AuthModal from './AuthModal';

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    login: vi.fn(),
    register: vi.fn(),
  }),
}));

vi.mock('@flash-mall/shared', () => ({
  api: vi.fn(),
}));

describe('AuthModal', () => {
  it('associates every visible field with its label', () => {
    render(<AuthModal open onClose={vi.fn()} />);

    expect(screen.getByLabelText('手机号')).toHaveAttribute('name', 'phone');
    expect(screen.getByLabelText('密码')).toHaveAttribute('name', 'password');

    fireEvent.click(screen.getByRole('button', { name: '注册账号' }));

    expect(screen.getByLabelText('手机号')).toHaveAttribute('name', 'phone');
    expect(screen.getByLabelText('验证码')).toHaveAttribute('name', 'code');
    expect(screen.getByLabelText('密码')).toHaveAttribute('name', 'password');
  });
});

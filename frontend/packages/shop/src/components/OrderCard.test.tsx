import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import OrderCard from './OrderCard';

describe('OrderCard', () => {
  it('uses the immutable image URL captured by the order snapshot', () => {
    render(<OrderCard
      order={{
        order_id: 'order-image-1',
        product_id: 999,
        product_name: '中文商品🧥',
        image_url: '/uploads/products/hash.webp',
        amount: 1,
        status: 0,
        status_text: '待支付',
        payable_amount_fen: 9900,
        create_time: '2026-07-23 12:00:00',
      }}
      onPay={vi.fn()}
      onConfirm={vi.fn()}
      onRefund={vi.fn()}
    />);

    expect(screen.getByRole('img', { name: '中文商品🧥' })).toHaveAttribute('src', '/uploads/products/hash.webp');
  });
});

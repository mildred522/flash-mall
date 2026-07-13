import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ProductDetailPage from './ProductDetailPage';

const item = {
  product_id: 100,
  name: '轻量通勤风衣',
  image_url: '/uploads/products/coat.webp',
  origin_price_fen: 12900,
  final_price_fen: 9900,
  promotion_tag: '限时价',
  stock_available: 12,
  merchant_id: 1000,
  merchant_name: '城市衣橱',
  merchant_logo: '/uploads/stores/1000/logo.webp',
};

describe('ProductDetailPage', () => {
  beforeEach(() => {
    history.replaceState({}, '', '/product/100');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        code: 'OK',
        data: { item, store_products: [{ ...item, product_id: 101, name: '同店针织衫' }] },
      }),
    }));
  });

  it('renders product and merchant context and exposes both actions', async () => {
    const onBuy = vi.fn();
    render(<ProductDetailPage productId={100} onBuy={onBuy} />);

    expect(await screen.findByRole('heading', { name: '轻量通勤风衣' })).toBeInTheDocument();
    expect(screen.getByText('城市衣橱')).toBeInTheDocument();
    expect(screen.getByText('同店针织衫')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '立即购买' }));
    expect(onBuy).toHaveBeenCalledWith(100);

    fireEvent.click(screen.getByRole('button', { name: /进入店铺/ }));
    expect(window.location.pathname).toBe('/store/1000');
  });
});

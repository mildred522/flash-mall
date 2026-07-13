import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import ProductCard from './ProductCard';

describe('ProductCard', () => {
  it('renders the API image and falls back to an icon on load failure', () => {
    render(<ProductCard product={{
      product_id: 100,
      name: '首发风衣',
      image_url: '/uploads/products/coat.webp',
      origin_price_fen: 12900,
      final_price_fen: 9900,
      promotion_tag: '限时价',
      stock_available: 10,
    }} onView={vi.fn()} onBuy={vi.fn()} />);

    const image = screen.getByRole('img', { name: '首发风衣' });
    expect(image).toHaveAttribute('src', '/uploads/products/coat.webp');
    expect(screen.queryByText('🧥')).not.toBeInTheDocument();

    fireEvent.error(image);
    expect(screen.getByText('🧥')).toBeInTheDocument();
  });

  it('keeps product, merchant and purchase actions independent', () => {
    const onView = vi.fn();
    const onStore = vi.fn();
    const onBuy = vi.fn();
    render(<ProductCard product={{
      product_id: 100,
      name: '首发风衣',
      image_url: '',
      origin_price_fen: 12900,
      final_price_fen: 9900,
      promotion_tag: '限时价',
      stock_available: 10,
      merchant_id: 1000,
      merchant_name: '城市衣橱',
    }} onView={onView} onStore={onStore} onBuy={onBuy} />);

    fireEvent.click(screen.getByRole('button', { name: '查看首发风衣' }));
    expect(onView).toHaveBeenCalledWith(100);
    expect(onStore).not.toHaveBeenCalled();
    expect(onBuy).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '进入城市衣橱' }));
    expect(onStore).toHaveBeenCalledWith(1000);
    expect(onBuy).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '立即购买' }));
    expect(onBuy).toHaveBeenCalledWith(100);
  });

  it('disables purchase when the item is sold out', () => {
    render(<ProductCard product={{
      product_id: 100,
      name: '售罄风衣',
      image_url: '',
      origin_price_fen: 12900,
      final_price_fen: 9900,
      promotion_tag: '',
      stock_available: 0,
    }} onView={vi.fn()} onBuy={vi.fn()} />);

    expect(screen.getByRole('button', { name: '已售罄' })).toBeDisabled();
  });
});

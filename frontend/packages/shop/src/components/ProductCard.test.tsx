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
    }} onBuy={vi.fn()} />);

    const image = screen.getByRole('img', { name: '首发风衣' });
    expect(image).toHaveAttribute('src', '/uploads/products/coat.webp');
    expect(screen.queryByText('🧥')).not.toBeInTheDocument();

    fireEvent.error(image);
    expect(screen.getByText('🧥')).toBeInTheDocument();
  });
});

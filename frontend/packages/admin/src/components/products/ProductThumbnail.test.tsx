import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import ProductThumbnail from './ProductThumbnail';

describe('ProductThumbnail', () => {
  it('replaces a missing remote image with a visible placeholder', () => {
    render(<ProductThumbnail src="/uploads/products/missing.png" alt="油画 商品图" />);

    fireEvent.error(screen.getByRole('img', { name: '油画 商品图' }));

    expect(screen.getByText('图片不可用')).toBeInTheDocument();
    expect(screen.queryByRole('img', { name: '油画 商品图' })).not.toBeInTheDocument();
  });

  it('retries rendering when the image source changes', () => {
    const { rerender } = render(<ProductThumbnail src="/uploads/products/missing.png" alt="油画 商品图" />);

    fireEvent.error(screen.getByRole('img', { name: '油画 商品图' }));
    rerender(<ProductThumbnail src="/uploads/products/restored.png" alt="油画 商品图" />);

    expect(screen.getByRole('img', { name: '油画 商品图' })).toHaveAttribute(
      'src',
      '/uploads/products/restored.png',
    );
    expect(screen.queryByText('图片不可用')).not.toBeInTheDocument();
  });
});

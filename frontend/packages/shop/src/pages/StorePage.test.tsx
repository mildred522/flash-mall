import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import StorePage from './StorePage';

describe('StorePage', () => {
  beforeEach(() => {
    history.replaceState({}, '', '/store/1000');
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      const data = url.includes('/products')
        ? {
            items: [{
              product_id: 100,
              name: '店铺限定风衣',
              image_url: '',
              origin_price_fen: 12900,
              final_price_fen: 9900,
              promotion_tag: '',
              stock_available: 6,
              merchant_id: 1000,
              merchant_name: '城市衣橱',
            }],
            total: 1,
            page: 1,
            page_size: 12,
          }
        : {
            merchant_id: 1000,
            merchant_name: '城市衣橱',
            logo_url: '',
            banner_url: '',
            description: '为城市生活挑选耐穿又利落的日常服装。',
            status: 1,
            product_count: 1,
          };
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ code: 'OK', data }),
      } as Response);
    }));
  });

  it('renders store identity and navigates from its product list', async () => {
    render(<StorePage merchantId={1000} onBuy={vi.fn()} />);

    expect(await screen.findByRole('heading', { name: '城市衣橱' })).toBeInTheDocument();
    expect(screen.getByText('为城市生活挑选耐穿又利落的日常服装。')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: /查看店铺限定风衣/ }));
    expect(window.location.pathname).toBe('/product/100');
  });
});

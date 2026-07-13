import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ProductsPage from './ProductsPage';

const mocks = vi.hoisted(() => ({ authed: vi.fn(), uploadProductImage: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
  uploadProductImage: mocks.uploadProductImage,
}));

const product = {
  product_id: 100,
  name: '商家风衣',
  image_url: '/products/100.svg',
  origin_price_fen: 12900,
  sale_price_fen: 9900,
  supplier_id: 200,
  supplier_name: '供应商',
  stock_available: 50,
  promotion_price_fen: 0,
  promotion_type: '',
  promotion_tag: '',
  status: 1,
  status_text: 'active',
};

describe('Merchant ProductsPage', () => {
  beforeEach(() => {
    mocks.authed.mockReset();
    mocks.uploadProductImage.mockReset();
    mocks.uploadProductImage.mockResolvedValue('/uploads/products/merchant.png');
    mocks.authed.mockImplementation(async (path: string) => {
      if (path.startsWith('/api/admin/')) throw new Error(`merchant page called admin API: ${path}`);
      if (path.startsWith('/api/merchant/products?')) return { ok: true, status: 200, data: { items: [product], total: 1 } };
      if (path === '/api/merchant/products/create') return { ok: true, status: 200, data: { product_id: 901 } };
      if (path === '/api/merchant/products/stock-adjust') return { ok: true, status: 200, data: { product_id: 100, stock_available: 55 } };
      return { ok: true, status: 200, data: { ok: true } };
    });
  });

  it('creates a product with an uploaded image through merchant APIs only', async () => {
    const user = userEvent.setup();
    render(<ProductsPage />);
    await screen.findByText('商家风衣');
    await user.click(screen.getByRole('button', { name: /新增商品/ }));
    await user.type(screen.getByLabelText('商品名称'), '商家图片商品');
    await user.upload(screen.getByLabelText('上传图片'), new File(['png'], 'merchant.png', { type: 'image/png' }));
    for (const [label, value] of [['原价(分)', '1000'], ['售价(分)', '800'], ['供应商ID', '200'], ['初始库存', '10']] as const) {
      const input = screen.getByLabelText(label);
      await user.clear(input);
      await user.type(input, value);
    }
    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => expect(mocks.uploadProductImage).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'merchant.png' }),
      '/api/merchant/products/image',
    ));
    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/merchant/products/create',
      expect.objectContaining({ jsonBody: expect.objectContaining({ image_url: '/uploads/products/merchant.png' }) }),
    ));
    expect(mocks.authed.mock.calls.flat().some((value) => String(value).startsWith('/api/admin/'))).toBe(false);
  });

  it('adjusts stock through the merchant Kitex command endpoint', async () => {
    const user = userEvent.setup();
    render(<ProductsPage />);
    await screen.findByText('商家风衣');
    await user.click(screen.getAllByRole('button', { name: '调整库存' })[0]);
    const delta = screen.getByLabelText('库存变化量');
    await user.clear(delta);
    await user.type(delta, '5');
    await user.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/merchant/products/stock-adjust',
      expect.objectContaining({ jsonBody: { product_id: 100, delta: 5, bucket_idx: 0 } }),
    ));
  });
});

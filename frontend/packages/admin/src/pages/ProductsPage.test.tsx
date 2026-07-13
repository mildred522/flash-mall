import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ProductsPage from './ProductsPage';

const mocks = vi.hoisted(() => ({
  authed: vi.fn(),
  uploadProductImage: vi.fn(),
}));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
  uploadProductImage: mocks.uploadProductImage,
}));

describe('ProductsPage image fields', () => {
  beforeEach(() => {
    mocks.authed.mockReset();
    mocks.uploadProductImage.mockReset();
    mocks.uploadProductImage.mockResolvedValue('/uploads/products/admin.png');
    mocks.authed.mockImplementation(async (path: string, options?: { jsonBody?: unknown }) => {
      if (path.startsWith('/api/admin/suppliers')) {
        return {
          ok: true,
          status: 200,
          data: { items: [{ supplier_id: 200, name: '平台供应商', status: 1 }], total: 1 },
        };
      }
      if (path.startsWith('/api/admin/products?')) {
        return { ok: true, status: 200, data: { items: [], total: 0 } };
      }
      if (path === '/api/admin/products/create') {
        return { ok: true, status: 200, data: { product_id: 901, body: options?.jsonBody } };
      }
      return { ok: true, status: 200, data: {} };
    });
  });

  it('uploads a selected image and includes its URL in product creation', async () => {
    const user = userEvent.setup();
    render(<ProductsPage />);
    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(expect.stringContaining('/api/admin/suppliers')));

    await user.click(await screen.findByRole('button', { name: /新增商品/ }));
    expect(screen.getByLabelText('图片地址')).toBeInTheDocument();
    const imageInput = screen.getByLabelText('上传图片');
    await user.upload(imageInput, new File(['png'], 'admin.png', { type: 'image/png' }));
    await user.type(screen.getByLabelText('商品名称'), '图片商品');
    const originPrice = screen.getByLabelText('原价(分)');
    const salePrice = screen.getByLabelText('售价(分)');
    await user.clear(originPrice);
    await user.type(originPrice, '1000');
    await user.clear(salePrice);
    await user.type(salePrice, '800');
    await user.click(screen.getByRole('button', { name: '确 定' }));

    await waitFor(() => expect(mocks.uploadProductImage).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'admin.png' }),
      '/api/admin/products/image',
    ));
    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/admin/products/create',
      expect.objectContaining({
        jsonBody: expect.objectContaining({ image_url: '/uploads/products/admin.png' }),
      }),
    ));
  });
});

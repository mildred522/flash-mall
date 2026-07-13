import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import StoreSettingsPage from './StoreSettingsPage';

const mocks = vi.hoisted(() => ({ authed: vi.fn(), uploadImageAsset: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
  uploadImageAsset: mocks.uploadImageAsset,
}));

const initialProfile = {
  merchant_id: 1000,
  merchant_name: '城市衣橱',
  logo_url: '',
  banner_url: '',
  description: '',
  version: 0,
};

describe('StoreSettingsPage', () => {
  beforeEach(() => {
    mocks.authed.mockReset();
    mocks.uploadImageAsset.mockReset();
    mocks.uploadImageAsset
      .mockResolvedValueOnce('/uploads/stores/1000/logo.webp')
      .mockResolvedValueOnce('/uploads/stores/1000/banner.webp');
    mocks.authed.mockImplementation(async (_path: string, options?: { method?: string }) => {
      if (options?.method === 'POST') {
        return {
          ok: true,
          status: 200,
          data: {
            ...initialProfile,
            logo_url: '/uploads/stores/1000/logo.webp',
            banner_url: '/uploads/stores/1000/banner.webp',
            description: '为城市生活挑选耐穿又利落的日常服装。',
            version: 1,
          },
        };
      }
      return { ok: true, status: 200, data: initialProfile };
    });
  });

  it('uploads both visual assets and saves the first profile version', async () => {
    const user = userEvent.setup();
    const { container } = render(<StoreSettingsPage />);
    expect(await screen.findByRole('heading', { name: '店铺设置' })).toBeInTheDocument();
    expect(screen.getByText('当前版本 0')).toBeInTheDocument();

    const logoInput = screen.getByTestId('logo-upload').querySelector('input[type="file"]') as HTMLInputElement;
    const bannerInput = screen.getByTestId('banner-upload').querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(logoInput, new File(['logo'], 'logo.webp', { type: 'image/webp' }));
    await user.upload(bannerInput, new File(['banner'], 'banner.webp', { type: 'image/webp' }));

    const description = screen.getByLabelText('店铺简介');
    await user.type(description, '为城市生活挑选耐穿又利落的日常服装。');
    expect(screen.getByTestId('description-count')).toHaveTextContent('18 / 300');
    expect(container.querySelector('img[src="/uploads/stores/1000/logo.webp"]')).toBeInTheDocument();
    expect(container.querySelector('[style*="banner.webp"]')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /保存店铺资料/ }));
    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/merchant/store/profile',
      expect.objectContaining({
        method: 'POST',
        jsonBody: {
          logo_url: '/uploads/stores/1000/logo.webp',
          banner_url: '/uploads/stores/1000/banner.webp',
          description: '为城市生活挑选耐穿又利落的日常服装。',
          expected_version: 0,
        },
      }),
    ));
    expect(await screen.findByText('店铺资料已保存')).toBeInTheDocument();
    expect(screen.getByText('当前版本 1')).toBeInTheDocument();
  });

  it('keeps edits and explains an optimistic-lock conflict', async () => {
    const user = userEvent.setup();
    mocks.authed.mockImplementation(async (_path: string, options?: { method?: string }) => options?.method === 'POST'
      ? { ok: false, status: 409, data: { message: 'version conflict' } }
      : { ok: true, status: 200, data: { ...initialProfile, version: 3 } });

    render(<StoreSettingsPage />);
    const description = await screen.findByLabelText('店铺简介');
    await user.type(description, '这段编辑不能丢失');
    await user.click(screen.getByRole('button', { name: /保存店铺资料/ }));

    expect(await screen.findByText('资料已被更新，请刷新后重试')).toBeInTheDocument();
    expect(description).toHaveValue('这段编辑不能丢失');
  });
});

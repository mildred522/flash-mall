import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ShowcasePage, { moveSlot, normalizeShowcaseSlots } from './ShowcasePage';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

const product = (productId: number, merchantId: number, name = `商品${productId}`) => ({
  product_id: productId,
  name,
  image_url: '',
  origin_price_fen: 12900,
  final_price_fen: 9900,
  promotion_tag: '限时价',
  stock_available: 12,
  merchant_id: merchantId,
  merchant_name: `商家${merchantId}`,
});

const layout = {
  version: 7,
  operator_id: 1,
  publish_time: '2026-07-13 20:00:00',
  items: [
    { slot_no: 1, product_id: 100, empty: false, valid: true, product: product(100, 1000, '风衣') },
    { slot_no: 2, product_id: 101, empty: false, valid: false, invalid_reason: 'out_of_stock', product: product(101, 1000, '针织衫') },
  ],
};

const candidates = {
  items: [
    { product: product(100, 1000, '风衣'), score: 92, sales_7d: 80, sales_score: 30, stock_score: 20, promotion_score: 15, freshness_score: 12, diversity_score: 15, reasons: ['近七日销量领先'] },
    { product: product(102, 1000, '同商家第三件'), score: 88, sales_7d: 70, sales_score: 28, stock_score: 18, promotion_score: 15, freshness_score: 12, diversity_score: 15, reasons: ['库存充足'] },
    { product: product(200, 2000, '手工皮包'), score: 84, sales_7d: 60, sales_score: 25, stock_score: 17, promotion_score: 15, freshness_score: 12, diversity_score: 15, reasons: ['商家多样性'] },
  ],
  total: 3,
  page: 1,
  page_size: 50,
};

describe('showcase slot model', () => {
  it('normalizes to twelve slots and keeps accessible reorder semantics', () => {
    const slots = normalizeShowcaseSlots(layout.items);
    expect(slots).toHaveLength(12);
    expect(slots[2]).toMatchObject({ slot_no: 3, product_id: 0, empty: true });
    const moved = moveSlot(slots, 0, 2);
    expect(moved[2].product_id).toBe(100);
    expect(moved.map((slot) => slot.slot_no)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]);
  });
});

describe('ShowcasePage', () => {
  beforeEach(() => {
    mocks.authed.mockReset();
    mocks.authed.mockImplementation(async (path: string, options?: { method?: string; jsonBody?: unknown }) => {
      if (path === '/api/admin/showcase') return { ok: true, status: 200, data: layout };
      if (path.startsWith('/api/admin/showcase/candidates')) return { ok: true, status: 200, data: candidates };
      if (path === '/api/admin/showcase/publish' && options?.method === 'POST') {
        return { ok: true, status: 200, data: { ...layout, version: 8, items: (options.jsonBody as { items: unknown[] }).items } };
      }
      throw new Error(`unexpected API ${path}`);
    });
  });

  it('edits twelve slots with duplicate and merchant diversity guards, then publishes non-empty items', async () => {
    const user = userEvent.setup();
    render(<ShowcasePage />);

    expect(await screen.findByRole('heading', { name: '首页橱窗' })).toBeInTheDocument();
    expect(screen.getAllByTestId(/^showcase-slot-/)).toHaveLength(12);
    expect(screen.getByText('库存不足或已售罄')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /加入风衣/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /加入同商家第三件/ })).toBeDisabled();
    expect(screen.getByText('仅建议，不会自动发布')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /加入手工皮包/ }));
    expect(withinSlot(3).getByText('手工皮包')).toBeInTheDocument();

    await user.click(withinSlot(3).getByRole('button', { name: '上移' }));
    expect(withinSlot(2).getByText('手工皮包')).toBeInTheDocument();
    await user.click(withinSlot(2).getByRole('button', { name: '移除' }));
    expect(withinSlot(2).getByText('空槽位')).toBeInTheDocument();

    fireEvent.dragStart(screen.getByTestId('showcase-slot-1'));
    fireEvent.drop(screen.getByTestId('showcase-slot-3'));
    expect(withinSlot(3).getByText('风衣')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /发布首页橱窗/ }));
    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/admin/showcase/publish',
      expect.objectContaining({
        method: 'POST',
        jsonBody: expect.objectContaining({
          expected_version: 7,
          items: expect.arrayContaining([expect.objectContaining({ product_id: 100 })]),
        }),
      }),
    ));
    const publishCall = mocks.authed.mock.calls.find(([path]) => path === '/api/admin/showcase/publish');
    expect(publishCall[1].jsonBody.items.every((item: { product_id: number }) => item.product_id > 0)).toBe(true);
    expect(await screen.findByText('橱窗已发布，当前版本 8')).toBeInTheDocument();
  });

  it('keeps the draft after a publish version conflict', async () => {
    const user = userEvent.setup();
    mocks.authed.mockImplementation(async (path: string) => {
      if (path === '/api/admin/showcase') return { ok: true, status: 200, data: layout };
      if (path.startsWith('/api/admin/showcase/candidates')) return { ok: true, status: 200, data: candidates };
      return { ok: false, status: 409, data: { message: 'version conflict' } };
    });
    render(<ShowcasePage />);
    await screen.findByRole('heading', { name: '首页橱窗' });
    expect(withinSlot(1).getByText('风衣')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /发布首页橱窗/ }));
    expect(await screen.findByText('橱窗已被其他管理员更新，请刷新后重试')).toBeInTheDocument();
    expect(withinSlot(1).getByText('风衣')).toBeInTheDocument();
  });
});

function withinSlot(slotNo: number) {
  return within(screen.getByTestId(`showcase-slot-${slotNo}`));
}

import type { ShowcaseCandidate, ShowcaseSlot } from '@flash-mall/shared';

export type DraftSlot = ShowcaseSlot & { dirty?: boolean };

export const invalidShowcaseReasonText: Record<string, string> = {
  product_not_found: '商品不存在', product_inactive: '商品已下架',
  merchant_not_found: '所属商家不存在', merchant_inactive: '所属店铺已停用',
  out_of_stock: '库存不足或已售罄',
};

export function normalizeShowcaseSlots(items: ShowcaseSlot[] = []): DraftSlot[] {
  const bySlot = new Map(items.map((item) => [item.slot_no, item]));
  return Array.from({ length: 12 }, (_, index) => {
    const slotNo = index + 1;
    const source = bySlot.get(slotNo);
    if (!source || source.product_id <= 0) {
      return { slot_no: slotNo, product_id: 0, empty: true, valid: true };
    }
    return { ...source, slot_no: slotNo, empty: false };
  });
}

export function moveSlot(items: DraftSlot[], from: number, to: number): DraftSlot[] {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) return items;
  const next = [...items];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next.map((slot, index) => ({ ...slot, slot_no: index + 1, dirty: true }));
}

export function candidateDisabledReason(candidate: ShowcaseCandidate, slots: DraftSlot[]): string {
  if (slots.some((slot) => slot.product_id === candidate.product.product_id)) return '该商品已在橱窗中';
  const merchantId = candidate.product.merchant_id;
  if (merchantId && slots.filter((slot) => slot.product?.merchant_id === merchantId).length >= 2) {
    return '同一商家最多展示 2 件商品';
  }
  return slots.some((slot) => slot.product_id <= 0) ? '' : '橱窗已满';
}

export function addShowcaseCandidate(slots: DraftSlot[], candidate: ShowcaseCandidate): DraftSlot[] {
  if (candidateDisabledReason(candidate, slots)) return slots;
  const emptyIndex = slots.findIndex((slot) => slot.product_id <= 0);
  if (emptyIndex < 0) return slots;
  return slots.map((slot, index) => index === emptyIndex ? {
    slot_no: slot.slot_no, product_id: candidate.product.product_id, product: candidate.product,
    empty: false, valid: true, dirty: true,
  } : slot);
}

export function removeShowcaseSlot(slots: DraftSlot[], index: number): DraftSlot[] {
  return slots.map((slot, slotIndex) => slotIndex === index ? {
    slot_no: slot.slot_no, product_id: 0, empty: true, valid: true, dirty: true,
  } : slot);
}

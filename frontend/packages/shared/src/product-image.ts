import { PRODUCT_META } from './constants';

export function resolveProductImage(productId: number, imageURL?: string): string {
  const explicit = imageURL?.trim();
  return explicit || PRODUCT_META[productId]?.image || '';
}

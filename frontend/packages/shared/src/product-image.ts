import { PRODUCT_META } from './constants';

export const PRODUCT_IMAGE_FALLBACK_DATA_URI = `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(`
<svg xmlns="http://www.w3.org/2000/svg" width="160" height="120" viewBox="0 0 160 120">
  <rect width="160" height="120" fill="#f5f5f5"/>
  <path d="M42 78l22-25 18 18 13-14 23 21H42z" fill="#d9d9d9"/>
  <circle cx="104" cy="39" r="9" fill="#d9d9d9"/>
  <text x="80" y="102" text-anchor="middle" font-size="13" fill="#8c8c8c">图片不可用</text>
</svg>`)}`;

export function resolveProductImage(productId: number, imageURL?: string): string {
  const explicit = imageURL?.trim();
  return explicit || PRODUCT_META[productId]?.image || '';
}

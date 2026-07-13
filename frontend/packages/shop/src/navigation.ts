export type ShopRoute =
  | { page: 'shop' }
  | { page: 'orders' }
  | { page: 'product'; productId: number }
  | { page: 'store'; merchantId: number };

export function parseShopRoute(pathname: string): ShopRoute {
  if (pathname === '/' || pathname === '/shop') return { page: 'shop' };
  if (pathname === '/orders') return { page: 'orders' };

  const productMatch = pathname.match(/^\/product\/(\d+)\/?$/);
  if (productMatch) return { page: 'product', productId: Number(productMatch[1]) };

  const storeMatch = pathname.match(/^\/store\/(\d+)\/?$/);
  if (storeMatch) return { page: 'store', merchantId: Number(storeMatch[1]) };

  return { page: 'shop' };
}

export function navigateShop(path: string) {
  if (window.location.pathname !== path) window.history.pushState({}, '', path);
  window.dispatchEvent(new Event('flash-shop:navigate'));
}

export const STATUS_MAP: Record<number, { text: string; cls: string }> = {
  0: { text: '待支付', cls: 'pending' },
  1: { text: '已支付', cls: 'paid' },
  2: { text: '已关闭', cls: 'closed' },
  3: { text: '已发货', cls: 'shipped' },
  4: { text: '已收货', cls: 'completed' },
  5: { text: '退款中', cls: 'refund' },
  6: { text: '已退款', cls: 'refunded' },
};

export function formatPriceFen(fen: number): string {
  return (fen / 100).toFixed(2);
}

export interface ProductMeta {
  icon: string;
  image?: string;
  desc: string;
}

export const PRODUCT_META: Record<number, ProductMeta> = {
  100: { icon: '🧥', image: '/products/100.svg', desc: '首发风衣' },
  101: { icon: '🪶', image: '/products/101.svg', desc: '轻薄羽绒服' },
  102: { icon: '👕', image: '/products/102.svg', desc: '纯棉T恤三件套' },
  103: { icon: '👟', image: '/products/103.svg', desc: '运动休闲鞋' },
  104: { icon: '🔋', image: '/products/104.svg', desc: '便携充电宝' },
};

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
  100: { icon: '🧥', desc: '首发风衣' },
  101: { icon: '👟', desc: '限量版运动鞋' },
  102: { icon: '👕', desc: '纯棉T恤经典款' },
  103: { icon: '🎒', desc: '高端双肩背包' },
  104: { icon: '🕶️', desc: '时尚太阳眼镜' },
};

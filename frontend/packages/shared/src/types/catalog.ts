export interface ProductCard {
  product_id: number;
  name: string;
  image_url: string;
  origin_price_fen: number;
  final_price_fen: number;
  promotion_tag: string;
  stock_available: number;
  merchant_id?: number;
  merchant_name?: string;
  merchant_logo?: string;
  store_url?: string;
  store_status?: number;
  slot_no?: number;
}

export interface CatalogResp {
  items: ProductCard[];
  total: number;
  page: number;
  page_size: number;
}

export interface MerchantStoreProfile {
  merchant_id: number;
  merchant_name: string;
  logo_url: string;
  banner_url: string;
  description: string;
  version: number;
}

export interface PublicStoreDetail {
  merchant_id: number;
  merchant_name: string;
  logo_url: string;
  banner_url: string;
  description: string;
  status: number;
  product_count: number;
}

export interface StoreProductsResp {
  items: ProductCard[];
  total: number;
  page: number;
  page_size: number;
}

export interface ProductDetailResp {
  item: ProductCard;
  store_products: ProductCard[];
}

export type ShowcaseInvalidReason =
  | 'product_not_found'
  | 'product_inactive'
  | 'merchant_not_found'
  | 'merchant_inactive'
  | 'out_of_stock';

export interface ShowcaseSlot {
  slot_no: number;
  product_id: number;
  empty: boolean;
  valid: boolean;
  invalid_reason?: ShowcaseInvalidReason;
  product?: ProductCard;
}

export interface ShowcaseResp {
  version: number;
  operator_id: number;
  publish_time: string;
  items: ShowcaseSlot[];
}

export interface ShowcaseCandidate {
  product: ProductCard;
  score: number;
  sales_7d: number;
  sales_score: number;
  stock_score: number;
  promotion_score: number;
  freshness_score: number;
  diversity_score: number;
  reasons: string[];
}

export interface ShowcaseCandidatesResp {
  items: ShowcaseCandidate[];
  total: number;
  page: number;
  page_size: number;
}

export interface ShowcasePublishReq {
  expected_version: number;
  items: Array<{ slot_no: number; product_id: number }>;
}

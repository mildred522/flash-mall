package handler

import (
	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
	"flash-mall/app/gateway/hertz/internal/application/productcommand"
)

type ProductCard struct {
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name"`
	ImageURL       string `json:"image_url"`
	OriginPriceFen int64  `json:"origin_price_fen"`
	FinalPriceFen  int64  `json:"final_price_fen"`
	SupplierID     int64  `json:"supplier_id"`
	SupplierName   string `json:"supplier_name"`
	PromotionTag   string `json:"promotion_tag"`
	StockAvailable int64  `json:"stock_available"`
	StockReserved  int64  `json:"stock_reserved,omitempty"`
	StockTotal     int64  `json:"stock_total,omitempty"`
	StockSource    string `json:"stock_source,omitempty"`
	MerchantID     int64  `json:"merchant_id"`
	MerchantName   string `json:"merchant_name"`
	MerchantLogo   string `json:"merchant_logo,omitempty"`
	StoreURL       string `json:"store_url"`
	StoreStatus    int64  `json:"store_status"`
	SlotNo         int64  `json:"slot_no,omitempty"`
}

type ProductListResp struct {
	Items    []ProductCard `json:"items"`
	Total    int64         `json:"total"`
	Page     int64         `json:"page"`
	PageSize int64         `json:"page_size"`
}

type PublicStoreDetail = catalogquery.StoreDetail

type StoreProductListResp struct {
	Items    []ProductCard `json:"items"`
	Total    int64         `json:"total"`
	Page     int64         `json:"page"`
	PageSize int64         `json:"page_size"`
}

type ProductDetailResp struct {
	Item          ProductCard   `json:"item"`
	StoreProducts []ProductCard `json:"store_products"`
}

type ShowcaseSlot struct {
	SlotNo        int64        `json:"slot_no"`
	ProductID     int64        `json:"product_id"`
	Empty         bool         `json:"empty"`
	Valid         bool         `json:"valid"`
	InvalidReason string       `json:"invalid_reason,omitempty"`
	Product       *ProductCard `json:"product,omitempty"`
}

type ShowcaseResp struct {
	Version     int64          `json:"version"`
	OperatorID  int64          `json:"operator_id"`
	PublishTime string         `json:"publish_time"`
	Items       []ShowcaseSlot `json:"items"`
}

type ShowcaseCandidate struct {
	Product        ProductCard `json:"product"`
	Score          int         `json:"score"`
	Sales7d        int64       `json:"sales_7d"`
	SalesScore     int         `json:"sales_score"`
	StockScore     int         `json:"stock_score"`
	PromotionScore int         `json:"promotion_score"`
	FreshnessScore int         `json:"freshness_score"`
	DiversityScore int         `json:"diversity_score"`
	Reasons        []string    `json:"reasons"`
}

type ShowcaseCandidatesResp struct {
	Items    []ShowcaseCandidate `json:"items"`
	Total    int64               `json:"total"`
	Page     int64               `json:"page"`
	PageSize int64               `json:"page_size"`
}

type AdminProductItem = catalogquery.AdminProduct

type AdminProductListResp struct {
	Items    []AdminProductItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int64              `json:"page"`
	PageSize int64              `json:"page_size"`
}

type AdminProductCardSnapshotRefreshReq struct {
	ProductID     int64 `json:"product_id,omitempty"`
	Limit         int64 `json:"limit,omitempty"`
	WindowMinutes int64 `json:"window_minutes,omitempty"`
}

type AdminProductCardSnapshotRefreshResp struct {
	ProductCount  int64 `json:"product_count"`
	Affected      int64 `json:"affected"`
	Limit         int64 `json:"limit"`
	WindowMinutes int64 `json:"window_minutes"`
}

type AdminStockSnapshotRebuildReq struct {
	ProductID int64 `json:"product_id,omitempty"`
	Limit     int64 `json:"limit,omitempty"`
}

type AdminProductUpdateReq = productcommand.UpdateInput

type AdminProductCreateReq = productcommand.CreateInput

type AdminProductCreateResp struct {
	ProductID int64 `json:"product_id"`
}

type ProductInventorySeedRetryReq struct {
	ProductID int64 `json:"product_id"`
}

type ProductInventorySeedRetryResp struct {
	ProductID int64  `json:"product_id"`
	Status    string `json:"status"`
}

type AdminProductStockAdjustReq struct {
	ProductID int64 `json:"product_id"`
	Delta     int64 `json:"delta"`
	BucketIdx int64 `json:"bucket_idx,omitempty"`
}

type AdminProductStockAdjustResp struct {
	ProductID      int64 `json:"product_id"`
	StockAvailable int64 `json:"stock_available"`
}

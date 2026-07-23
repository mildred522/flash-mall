package catalogquery

import (
	"context"

	"flash-mall/app/common/apperror"
)

type ProductMeta struct {
	ImageURL      string
	SupplierName  string
	MerchantID    int64
	MerchantName  string
	MerchantLogo  string
	StoreStatus   int64
	ProductStatus int64
}

type ListQuery struct {
	Page, PageSize                                        int64
	Keyword                                               string
	ProductID, MerchantID, SupplierID, CategoryID, Status int64
}

type IDPage struct {
	ProductIDs []int64
	Total      int64
}

type StoreDetail struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	LogoURL      string `json:"logo_url"`
	BannerURL    string `json:"banner_url"`
	Description  string `json:"description"`
	Status       int64  `json:"status"`
	ProductCount int64  `json:"product_count"`
}

type AdminProduct struct {
	ProductID         int64  `json:"product_id"`
	MerchantID        int64  `json:"merchant_id"`
	MerchantName      string `json:"merchant_name"`
	Name              string `json:"name"`
	ImageURL          string `json:"image_url"`
	OriginPriceFen    int64  `json:"origin_price_fen"`
	SalePriceFen      int64  `json:"sale_price_fen"`
	SupplierID        int64  `json:"supplier_id"`
	SupplierName      string `json:"supplier_name"`
	StockAvailable    int64  `json:"stock_available"`
	PromotionPriceFen int64  `json:"promotion_price_fen"`
	Status            int64  `json:"status"`
	StatusText        string `json:"status_text"`
	PromotionText     string `json:"promotion_text"`
}

type Repository interface {
	ProductMetadata(context.Context, []int64) (map[int64]ProductMeta, error)
	ProductIDs(context.Context, ListQuery) (IDPage, error)
	StoreProductIDs(context.Context, int64, string, int64, int64) (IDPage, error)
	StoreDetail(context.Context, int64) (StoreDetail, bool, error)
	AdminProducts(context.Context, ListQuery) ([]AdminProduct, int64, error)
	AdminProductDetail(context.Context, int64) (AdminProduct, bool, error)
	OwnsProduct(context.Context, int64, int64) (bool, error)
}

func (s *Service) AdminProducts(ctx context.Context, query ListQuery) ([]AdminProduct, int64, error) {
	items, total, err := s.repository.AdminProducts(ctx, query)
	decorateAdminProducts(items)
	return items, total, err
}

func (s *Service) AdminProductDetail(ctx context.Context, productID int64) (AdminProduct, error) {
	item, found, err := s.repository.AdminProductDetail(ctx, productID)
	if err != nil {
		return AdminProduct{}, err
	}
	if !found {
		return AdminProduct{}, apperror.New(apperror.CodeProductNotFound, "product not found")
	}
	decorateAdminProducts([]AdminProduct{item})
	return item, nil
}

func (s *Service) OwnsProduct(ctx context.Context, merchantID, productID int64) (bool, error) {
	if merchantID <= 0 || productID <= 0 {
		return false, nil
	}
	return s.repository.OwnsProduct(ctx, merchantID, productID)
}

func decorateAdminProducts(items []AdminProduct) {
	for index := range items {
		switch items[index].Status {
		case 1:
			items[index].StatusText = "上架"
		case 2:
			items[index].StatusText = "下架"
		default:
			items[index].StatusText = "未知"
		}
		if items[index].PromotionPriceFen > 0 {
			items[index].PromotionText = "限时价"
		} else {
			items[index].PromotionText = "无活动"
		}
	}
}

func (s *Service) StoreDetail(ctx context.Context, merchantID int64) (StoreDetail, error) {
	detail, found, err := s.repository.StoreDetail(ctx, merchantID)
	if err != nil {
		return StoreDetail{}, err
	}
	if !found {
		return StoreDetail{}, apperror.New(apperror.CodeMerchantNotFound, "merchant store not found")
	}
	return detail, nil
}

func (s *Service) ProductIDs(ctx context.Context, query ListQuery) (IDPage, error) {
	return s.repository.ProductIDs(ctx, query)
}

func (s *Service) StoreProductIDs(ctx context.Context, merchantID int64, keyword string, page, pageSize int64) (IDPage, error) {
	return s.repository.StoreProductIDs(ctx, merchantID, keyword, page, pageSize)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) ProductMetadata(ctx context.Context, productIDs []int64) (map[int64]ProductMeta, error) {
	if len(productIDs) == 0 {
		return map[int64]ProductMeta{}, nil
	}
	return s.repository.ProductMetadata(ctx, productIDs)
}

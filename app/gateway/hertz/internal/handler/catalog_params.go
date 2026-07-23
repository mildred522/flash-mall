package handler

import (
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/catalogquery"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	defaultProductPage     int64 = 1
	defaultProductPageSize int64 = 20
	maxProductPageSize     int64 = 100
)

type productListQuery = catalogquery.ListQuery

func parseProductListQuery(c *app.RequestContext, activeOnly bool) (productListQuery, *apperror.Error) {
	page, err := parseInt64Default(c.Query("page"), defaultProductPage)
	if err != nil || page <= 0 {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page must be positive")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), defaultProductPageSize)
	if err != nil || pageSize <= 0 {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive")
	}
	if pageSize > maxProductPageSize {
		pageSize = maxProductPageSize
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric")
	}
	supplierID, err := parseOptionalInt64(c.Query("supplier_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "supplier_id must be numeric")
	}
	categoryID, err := parseOptionalInt64(c.Query("category_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "category_id must be numeric")
	}
	status := int64(1)
	if !activeOnly {
		status, err = parseInt64Default(c.Query("status"), -1)
		if err != nil {
			return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
		}
	}
	return productListQuery{Page: page, PageSize: pageSize, Keyword: strings.TrimSpace(c.Query("keyword")), ProductID: productID,
		MerchantID: merchantID, SupplierID: supplierID, CategoryID: categoryID, Status: status}, nil
}

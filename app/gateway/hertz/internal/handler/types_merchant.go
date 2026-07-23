package handler

import (
	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"
	"flash-mall/app/gateway/hertz/internal/application/merchantstore"
)

type MerchantMeItem = merchantquery.Merchant

type MerchantMeResp = merchantquery.MeResponse

type MerchantStoreProfile = merchantstore.Profile
type merchantStoreUpdateReq = merchantstore.UpdateInput

type MerchantApplyReq = merchantonboarding.SubmitInput

type MerchantApplyResp struct {
	ApplyID int64  `json:"apply_id"`
	Status  string `json:"status"`
}

type MerchantApplicationItem = merchantquery.Application

type MerchantApplicationResp = merchantquery.ApplicationResponse

type AdminMerchantApplicationListReq = merchantonboarding.ListQuery
type AdminMerchantApplicationItem = merchantonboarding.Application
type AdminMerchantApplicationListResp = merchantonboarding.ListResult

type MerchantDashboardStatsResp = merchantquery.DashboardStats

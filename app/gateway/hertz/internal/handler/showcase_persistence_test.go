package handler

import (
	"context"
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/adapters/productmysql"
	showcaseapp "flash-mall/app/gateway/hertz/internal/application/showcase"
)

func loadShowcaseLayout(ctx context.Context, db *sql.DB) (ShowcaseResp, error) {
	service := showcaseapp.NewService(productmysql.NewShowcaseRepository(db))
	layout, err := service.Load(ctx)
	if err != nil {
		return ShowcaseResp{}, err
	}
	return handlerShowcaseLayout(layout), nil
}

func publishShowcase(ctx context.Context, db *sql.DB, operatorID int64, request showcasePublishReq) (int64, error) {
	return showcaseapp.NewService(productmysql.NewShowcaseRepository(db)).Publish(ctx, operatorID, request)
}

func showcaseServiceForTest(db *sql.DB) *showcaseapp.Service {
	return showcaseapp.NewService(productmysql.NewShowcaseRepository(db))
}

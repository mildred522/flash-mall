package handler

import (
	"bytes"
	"context"

	"flash-mall/app/common/apperror"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

func MetricsHandler() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		families, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			fail(ctx, c, consts.StatusInternalServerError, apperror.Wrap(apperror.CodeInternal, "metrics gather failed", err))
			return
		}
		var body bytes.Buffer
		for _, family := range families {
			if _, err = expfmt.MetricFamilyToText(&body, family); err != nil {
				fail(ctx, c, consts.StatusInternalServerError, apperror.Wrap(apperror.CodeInternal, "metrics encode failed", err))
				return
			}
		}
		c.Response.Header.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		c.Response.SetStatusCode(consts.StatusOK)
		c.Response.SetBody(body.Bytes())
	}
}

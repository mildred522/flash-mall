package paymentprovider

import (
	"context"
	"errors"
	"strings"
)

type apiResponse struct {
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	SubCode string `json:"sub_code"`
	SubMsg  string `json:"sub_msg"`
}

func (a *Alipay) Query(ctx context.Context, outTradeNo string) (QueryResult, error) {
	outTradeNo = strings.TrimSpace(outTradeNo)
	if outTradeNo == "" {
		return QueryResult{}, errors.New("out trade number is required")
	}
	var response struct {
		apiResponse
		OutTradeNo  string `json:"out_trade_no"`
		TradeNo     string `json:"trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
	}
	if err := a.call(ctx, "alipay.trade.query", map[string]string{"out_trade_no": outTradeNo},
		"alipay_trade_query_response", &response); err != nil {
		return QueryResult{}, err
	}
	if response.Code != "10000" {
		return QueryResult{}, apiError(response.Code, response.Msg, response.SubCode, response.SubMsg)
	}
	amountFen, err := parseAmountFen(response.TotalAmount)
	if err != nil {
		return QueryResult{}, err
	}
	return QueryResult{
		OutTradeNo: response.OutTradeNo, TradeNo: response.TradeNo,
		Status: response.TradeStatus, AmountFen: amountFen,
	}, nil
}

func (a *Alipay) Close(ctx context.Context, outTradeNo string) error {
	outTradeNo = strings.TrimSpace(outTradeNo)
	if outTradeNo == "" {
		return errors.New("out trade number is required")
	}
	var response apiResponse
	if err := a.call(ctx, "alipay.trade.close", map[string]string{"out_trade_no": outTradeNo},
		"alipay_trade_close_response", &response); err != nil {
		return err
	}
	if response.Code != "10000" {
		return apiError(response.Code, response.Msg, response.SubCode, response.SubMsg)
	}
	return nil
}

func (a *Alipay) Refund(ctx context.Context, request RefundRequest) (RefundResult, error) {
	if strings.TrimSpace(request.OutTradeNo) == "" || strings.TrimSpace(request.RefundID) == "" || request.AmountFen <= 0 {
		return RefundResult{}, errors.New("out trade number, refund id, and positive amount are required")
	}
	biz := map[string]any{
		"out_trade_no":   request.OutTradeNo,
		"out_request_no": request.RefundID,
		"refund_amount":  amountYuan(request.AmountFen),
		"refund_reason":  strings.TrimSpace(request.Reason),
	}
	var response struct {
		apiResponse
		OutTradeNo string `json:"out_trade_no"`
		TradeNo    string `json:"trade_no"`
	}
	if err := a.call(ctx, "alipay.trade.refund", biz, "alipay_trade_refund_response", &response); err != nil {
		return RefundResult{}, err
	}
	if response.Code != "10000" {
		return RefundResult{}, apiError(response.Code, response.Msg, response.SubCode, response.SubMsg)
	}
	return RefundResult{
		OutTradeNo: response.OutTradeNo, TradeNo: response.TradeNo,
		RefundID: request.RefundID, Status: "SUCCESS",
	}, nil
}

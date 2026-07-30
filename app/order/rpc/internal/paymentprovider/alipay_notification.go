package paymentprovider

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (a *Alipay) VerifyNotification(values url.Values) (Notification, error) {
	if err := verifyNotification(a.config.AlipayPublicKey, values); err != nil {
		return Notification{}, err
	}
	appID := strings.TrimSpace(values.Get("app_id"))
	if appID != a.config.AppID {
		return Notification{}, errors.New("alipay notification app id mismatch")
	}
	outTradeNo := strings.TrimSpace(values.Get("out_trade_no"))
	tradeNo := strings.TrimSpace(values.Get("trade_no"))
	tradeStatus := strings.TrimSpace(values.Get("trade_status"))
	if outTradeNo == "" || tradeNo == "" || tradeStatus == "" {
		return Notification{}, errors.New("alipay notification is missing trade identity")
	}
	amountFen, err := parseAmountFen(values.Get("total_amount"))
	if err != nil {
		return Notification{}, err
	}
	eventID := strings.TrimSpace(values.Get("notify_id"))
	if eventID == "" {
		eventID = tradeNo + ":" + tradeStatus
	}
	return Notification{
		AppID: appID, EventID: eventID, OutTradeNo: outTradeNo, TradeNo: tradeNo,
		TradeStatus: tradeStatus, AmountFen: amountFen, Raw: values.Encode(),
	}, nil
}

func parseAmountFen(value string) (int64, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid alipay amount %q", value)
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || yuan < 0 {
		return 0, fmt.Errorf("invalid alipay amount %q", value)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("invalid alipay amount precision %q", value)
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	fen, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid alipay amount %q", value)
	}
	return yuan*100 + fen, nil
}

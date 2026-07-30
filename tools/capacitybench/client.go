package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type businessClient struct {
	baseURL   string
	phone     string
	password  string
	productID int64
	token     string
	http      *http.Client
	runID     string
	sequence  atomic.Int64
}

type apiEnvelope struct {
	Code    any             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	UserID      int64  `json:"user_id"`
}

type orderResponse struct {
	OrderID string `json:"order_id"`
	Status  any    `json:"status"`
}

type paymentResponse struct {
	OrderID        string `json:"order_id"`
	PaymentOrderID string `json:"payment_order_id"`
	QRURL          string `json:"qr_url"`
	Status         string `json:"status"`
}

func newBusinessClient(baseURL, phone, password string, productID int64) *businessClient {
	return &businessClient{
		baseURL: strings.TrimRight(baseURL, "/"), phone: phone, password: password,
		productID: productID, runID: fmt.Sprintf("%d", time.Now().UnixNano()),
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *businessClient) login(ctx context.Context) error {
	var response loginResponse
	status, err := c.doJSON(ctx, http.MethodPost, "/api/auth/login", map[string]any{
		"phone": c.phone, "password": c.password, "device_type": "capacitybench",
	}, false, &response)
	if err != nil {
		return fmt.Errorf("login status=%d: %w", status, err)
	}
	if response.AccessToken == "" || response.UserID <= 0 {
		return fmt.Errorf("login response is incomplete")
	}
	c.token = response.AccessToken
	return nil
}

func (c *businessClient) execute(ctx context.Context, scenario string, sequence int64) sample {
	startedAt := time.Now()
	status, err := c.executeScenario(ctx, scenario, sequence)
	return sample{Duration: time.Since(startedAt), StatusCode: status, Err: err}
}

func (c *businessClient) executeScenario(ctx context.Context, scenario string, sequence int64) (int, error) {
	switch scenario {
	case "read":
		return c.read(ctx, sequence)
	case "order-cycle":
		return c.orderCycle(ctx, sequence)
	case "payment-cycle":
		return c.paymentCycle(ctx, sequence)
	case "idempotency":
		return c.createOrder(ctx, "capacity-"+c.runID+"-idempotency")
	default:
		return 0, fmt.Errorf("unsupported scenario %q", scenario)
	}
}

func (c *businessClient) read(ctx context.Context, sequence int64) (int, error) {
	switch sequence % 10 {
	case 7, 8:
		return c.doJSON(ctx, http.MethodGet,
			fmt.Sprintf("/api/shop/products/detail?product_id=%d", c.productID), nil, false, nil)
	case 9:
		return c.doJSON(ctx, http.MethodGet, "/api/shop/stores/detail?merchant_id=1101", nil, false, nil)
	default:
		return c.doJSON(ctx, http.MethodGet, "/api/shop/catalog", nil, false, nil)
	}
}

func (c *businessClient) orderCycle(ctx context.Context, sequence int64) (int, error) {
	sequence = c.sequence.Add(1) - 1
	requestID := fmt.Sprintf("capacity-%s-order-%d", c.runID, sequence)
	var created orderResponse
	status, err := c.createOrderInto(ctx, requestID, &created)
	if err != nil {
		return status, err
	}
	return c.doJSON(ctx, http.MethodPost, "/api/order/cancel", map[string]any{
		"order_id": created.OrderID, "reason": "capacity lifecycle cleanup",
	}, true, nil)
}

func (c *businessClient) paymentCycle(ctx context.Context, sequence int64) (int, error) {
	sequence = c.sequence.Add(1) - 1
	requestID := fmt.Sprintf("capacity-%s-payment-%d", c.runID, sequence)
	var created orderResponse
	status, err := c.createOrderInto(ctx, requestID, &created)
	if err != nil {
		return status, err
	}
	var payment paymentResponse
	status, err = c.doJSON(ctx, http.MethodPost, "/api/order/pay",
		map[string]any{"order_id": created.OrderID}, true, &payment)
	if err != nil {
		return status, err
	}
	parsed, err := url.Parse(payment.QRURL)
	if err != nil || parsed.Query().Get("token") == "" {
		return status, fmt.Errorf("payment QR URL has no token")
	}
	return c.doJSON(ctx, http.MethodPost, "/api/payment/sandbox/confirm",
		map[string]any{"token": parsed.Query().Get("token")}, false, nil)
}

func (c *businessClient) createOrder(ctx context.Context, requestID string) (int, error) {
	return c.doJSON(ctx, http.MethodPost, "/api/order/create", map[string]any{
		"request_id": requestID, "user_id": 0, "product_id": c.productID, "amount": 1,
	}, true, nil)
}

func (c *businessClient) createOrderInto(ctx context.Context, requestID string, output *orderResponse) (int, error) {
	return c.doJSON(ctx, http.MethodPost, "/api/order/create", map[string]any{
		"request_id": requestID, "user_id": 0, "product_id": c.productID, "amount": 1,
	}, true, output)
}

func (c *businessClient) doJSON(
	ctx context.Context,
	method string,
	path string,
	body any,
	auth bool,
	output any,
) (int, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if auth {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("unexpected status %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}
	if output == nil || len(bytes.TrimSpace(payload)) == 0 {
		return response.StatusCode, nil
	}
	if err := decodeAPIResponse(payload, output); err != nil {
		return response.StatusCode, err
	}
	return response.StatusCode, nil
}

func decodeAPIResponse(payload []byte, output any) error {
	var envelope apiEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return json.Unmarshal(envelope.Data, output)
	}
	return json.Unmarshal(payload, output)
}

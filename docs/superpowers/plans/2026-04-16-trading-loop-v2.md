# Trading Loop V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a business-grade transaction loop for `flash-mall` covering product pricing display, order-time price snapshot, stock reservation, payment success callback, timeout close, and stock release.

**Architecture:** Keep `order-api` as the browser-facing BFF and storefront surface. Put transaction truth in `order-rpc`, extend `product-rpc` with product card and stock summary reads, and reuse the existing DTM SAGA plus outbox path for consistency-critical state changes. Keep supply-side work narrow: only enough schema and query support to explain replenishment and supplier relation later.

**Tech Stack:** Go, go-zero, MySQL, Redis, DTM SAGA, RabbitMQ outbox, server-rendered HTML, generated gRPC stubs

---

## File Structure

### Product And Pricing Read Path

- Modify: `app/product/rpc/product.proto`
- Regenerate: `app/product/rpc/product/*.pb.go`
- Regenerate: `app/product/rpc/productclient/product.go`
- Create: `app/product/rpc/internal/logic/getproductcardlogic.go`
- Create: `app/product/rpc/internal/logic/getproductcardlogic_test.go`
- Modify: `app/product/rpc/internal/server/productserver.go`
- Modify: `app/order/api/internal/types/types.go`
- Modify: `app/order/api/internal/svc/servicecontext.go`
- Create: `app/order/api/internal/handler/cataloghandler.go`
- Create: `app/order/api/internal/handler/cataloghandler_test.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/web/shop.html`
- Modify: `scripts/k8s/init-db.sql`

### Pricing Engine And Order Persistence

- Create: `app/order/rpc/internal/pricing/quote.go`
- Create: `app/order/rpc/internal/pricing/quote_test.go`
- Modify: `app/order/rpc/order.proto`
- Regenerate: `app/order/rpc/order/*.pb.go`
- Regenerate: `app/order/rpc/orderclient/order.go`
- Modify: `app/order/api/internal/types/types.go`
- Modify: `app/order/api/internal/logic/createorderlogic.go`
- Modify: `app/order/api/internal/logic/createorderlogic_test.go`
- Modify: `app/order/rpc/internal/logic/createOrderLogic.go`
- Create: `app/order/rpc/internal/logic/createOrderLogic_test.go`
- Modify: `app/order/rpc/internal/server/orderServer.go`
- Modify: `scripts/k8s/init-db.sql`

### Payment And Order Status

- Modify: `app/order/rpc/order.proto`
- Regenerate: `app/order/rpc/order/*.pb.go`
- Regenerate: `app/order/rpc/orderclient/order.go`
- Create: `app/order/rpc/internal/logic/markorderpaidlogic.go`
- Create: `app/order/rpc/internal/logic/markorderpaidlogic_test.go`
- Create: `app/order/rpc/internal/logic/getorderdetaillogic.go`
- Create: `app/order/rpc/internal/logic/getorderdetaillogic_test.go`
- Modify: `app/order/rpc/internal/server/orderServer.go`
- Create: `app/order/api/internal/handler/payorderhandler.go`
- Create: `app/order/api/internal/handler/payorderhandler_test.go`
- Create: `app/order/api/internal/handler/orderdetailhandler.go`
- Create: `app/order/api/internal/handler/orderdetailhandler_test.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/web/shop.html`

### Timeout Close And Compensation Hardening

- Modify: `app/order/api/job/closeorder.go`
- Create: `app/order/api/job/closeorder_test.go`
- Modify: `app/product/rpc/internal/logic/revertstocklogic.go`
- Create: `app/product/rpc/internal/logic/revertstocklogic_test.go`
- Modify: `app/order/rpc/internal/job/outbox_publisher.go`
- Modify: `scripts/k8s/init-db.sql`

### Demo And Verification

- Create: `docs/TRADING_LOOP_V2_20260416.md`

### Task 1: Build Product Cards And Pricing Input Read Path

**Files:**
- Modify: `app/product/rpc/product.proto`
- Regenerate: `app/product/rpc/product/*.pb.go`
- Regenerate: `app/product/rpc/productclient/product.go`
- Create: `app/product/rpc/internal/logic/getproductcardlogic.go`
- Create: `app/product/rpc/internal/logic/getproductcardlogic_test.go`
- Modify: `app/product/rpc/internal/server/productserver.go`
- Modify: `app/order/api/internal/types/types.go`
- Create: `app/order/api/internal/handler/cataloghandler.go`
- Create: `app/order/api/internal/handler/cataloghandler_test.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/web/shop.html`
- Modify: `scripts/k8s/init-db.sql`

- [ ] **Step 1: Write the failing product-card read test**

```go
func TestGetProductCardLogic_GetProductCard_UsesPromotionAndStockSummary(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	seedProductCardData(t, svcCtx.SqlConn)

	resp, err := NewGetProductCardLogic(context.Background(), svcCtx).GetProductCard(&product.GetProductCardReq{
		ProductId: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProductId != 100 || resp.Name == "" {
		t.Fatalf("unexpected card payload: %#v", resp)
	}
	if resp.OriginPriceFen != 12900 {
		t.Fatalf("unexpected origin price: %d", resp.OriginPriceFen)
	}
	if resp.FinalPriceFen != 9900 {
		t.Fatalf("expected limited price to win, got %d", resp.FinalPriceFen)
	}
	if resp.StockAvailable <= 0 {
		t.Fatalf("expected positive stock summary, got %d", resp.StockAvailable)
	}
}
```

- [ ] **Step 2: Run the focused product-rpc test and verify red**

Run: `go test ./app/product/rpc/internal/logic -run "GetProductCard" -count=1`

Expected: FAIL because the request type, RPC method, and logic do not exist yet.

- [ ] **Step 3: Extend the product RPC contract for read path**

```proto
message GetProductCardReq {
  int64 product_id = 1;
}

message GetProductCardResp {
  int64 product_id = 1;
  string name = 2;
  int64 origin_price_fen = 3;
  int64 final_price_fen = 4;
  string promotion_type = 5;
  string promotion_tag = 6;
  int64 stock_available = 7;
  int64 supplier_id = 8;
}

service Product {
  rpc GetProductCard(GetProductCardReq) returns (GetProductCardResp);
  rpc Deduct(DeductReq) returns(Empty);
  rpc DeductRollback(DeductReq) returns(Empty);
  rpc RevertStock(RevertStockReq) returns(RevertStockResp);
}
```

- [ ] **Step 4: Add product, promotion, and minimal supply schema**

```sql
ALTER TABLE mall_product.product
  ADD COLUMN origin_price_fen bigint NOT NULL DEFAULT 0,
  ADD COLUMN sale_price_fen bigint NOT NULL DEFAULT 0,
  ADD COLUMN status tinyint NOT NULL DEFAULT 1,
  ADD COLUMN supplier_id bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS promotion_rule (
  id bigint NOT NULL AUTO_INCREMENT,
  product_id bigint NOT NULL,
  type varchar(32) NOT NULL,
  discount_value bigint NOT NULL DEFAULT 0,
  threshold_amount bigint NOT NULL DEFAULT 0,
  starts_at timestamp NULL DEFAULT NULL,
  ends_at timestamp NULL DEFAULT NULL,
  status tinyint NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY ix_product_status_time (product_id, status, starts_at, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS supplier (
  id bigint NOT NULL AUTO_INCREMENT,
  name varchar(128) NOT NULL,
  status tinyint NOT NULL DEFAULT 1,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 5: Implement the product card read logic and pass the test**

```go
func (l *GetProductCardLogic) GetProductCard(in *product.GetProductCardReq) (*product.GetProductCardResp, error) {
	var row struct {
		ID             int64          `db:"id"`
		Name           string         `db:"name"`
		OriginPriceFen int64          `db:"origin_price_fen"`
		SalePriceFen   int64          `db:"sale_price_fen"`
		SupplierID     int64          `db:"supplier_id"`
		LimitedPrice   sql.NullInt64  `db:"limited_price_fen"`
	}
	query := `
SELECT p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id,
       MIN(CASE WHEN pr.type = 'LIMITED_PRICE' THEN pr.discount_value END) AS limited_price_fen
FROM product p
LEFT JOIN promotion_rule pr ON pr.product_id = p.id AND pr.status = 1 AND NOW() BETWEEN pr.starts_at AND pr.ends_at
WHERE p.id = ?
GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id`
	if err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &row, query, in.ProductId); err != nil {
		return nil, err
	}
	var stockRows []struct{ Stock int64 `db:"stock"` }
	if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &stockRows, "SELECT stock FROM product_stock_bucket WHERE product_id = ?", row.ID); err != nil {
		return nil, err
	}
	var stock int64
	for _, item := range stockRows {
		stock += item.Stock
	}
	finalPrice := row.SalePriceFen
	if row.LimitedPrice.Valid && row.LimitedPrice.Int64 > 0 {
		finalPrice = row.LimitedPrice.Int64
	}
	promotionType := ""
	if row.LimitedPrice.Valid {
		promotionType = "LIMITED_PRICE"
	}
	return &product.GetProductCardResp{
		ProductId:      row.ID,
		Name:           row.Name,
		OriginPriceFen: row.OriginPriceFen,
		FinalPriceFen:  finalPrice,
		PromotionType:  promotionType,
		StockAvailable: stock,
		SupplierId:     row.SupplierID,
	}, nil
}
```

- [ ] **Step 6: Expose catalog data through order-api and storefront**

```go
type ProductCard struct {
	ProductId      int64  `json:"product_id"`
	Name           string `json:"name"`
	OriginPriceFen int64  `json:"origin_price_fen"`
	FinalPriceFen  int64  `json:"final_price_fen"`
	PromotionTag   string `json:"promotion_tag"`
	StockAvailable int64  `json:"stock_available"`
}

func CatalogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items := make([]types.ProductCard, 0, 4)
		for _, productID := range []int64{100} {
			card, err := svcCtx.ProductRpc.GetProductCard(r.Context(), &product.GetProductCardReq{ProductId: productID})
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			items = append(items, types.ProductCard{
				ProductId:      card.ProductId,
				Name:           card.Name,
				OriginPriceFen: card.OriginPriceFen,
				FinalPriceFen:  card.FinalPriceFen,
				PromotionTag:   card.PromotionTag,
				StockAvailable: card.StockAvailable,
			})
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"items": items})
	}
}
```

- [ ] **Step 7: Run the narrow verification**

Run: `go test ./app/product/rpc/internal/logic ./app/order/api/internal/handler -run "GetProductCard|CatalogHandler" -count=1`

Expected: PASS

- [ ] **Step 8: Commit the read-path slice**

```powershell
git add app/product/rpc/product.proto app/product/rpc/internal/logic/getproductcardlogic.go app/product/rpc/internal/logic/getproductcardlogic_test.go app/product/rpc/internal/server/productserver.go app/order/api/internal/handler/cataloghandler.go app/order/api/internal/handler/cataloghandler_test.go app/order/api/internal/handler/routes.go app/order/api/internal/handler/web/shop.html scripts/k8s/init-db.sql
git commit -m "feat: add product card pricing read path"
```

### Task 2: Add Pricing Engine, Price Snapshot, And Payment Order Persistence

**Files:**
- Create: `app/order/rpc/internal/pricing/quote.go`
- Create: `app/order/rpc/internal/pricing/quote_test.go`
- Modify: `app/order/rpc/order.proto`
- Regenerate: `app/order/rpc/order/*.pb.go`
- Regenerate: `app/order/rpc/orderclient/order.go`
- Modify: `app/order/api/internal/types/types.go`
- Modify: `app/order/api/internal/logic/createorderlogic.go`
- Modify: `app/order/api/internal/logic/createorderlogic_test.go`
- Modify: `app/order/rpc/internal/logic/createOrderLogic.go`
- Create: `app/order/rpc/internal/logic/createOrderLogic_test.go`
- Modify: `app/order/rpc/internal/server/orderServer.go`
- Modify: `scripts/k8s/init-db.sql`

- [ ] **Step 1: Write the failing pricing-engine tests**

```go
func TestQuote_BuildsLimitedPriceSnapshot(t *testing.T) {
	quote := BuildQuote(PricingInput{
		ProductID:       100,
		ProductName:     "首发风衣",
		Quantity:        2,
		OriginUnitPrice: 12900,
		LimitedPriceFen: 9900,
	})
	if quote.PayableAmountFen != 19800 {
		t.Fatalf("unexpected payable amount: %d", quote.PayableAmountFen)
	}
	if quote.DiscountAmountFen != 6000 {
		t.Fatalf("unexpected discount amount: %d", quote.DiscountAmountFen)
	}
	if quote.PromotionType != "LIMITED_PRICE" {
		t.Fatalf("unexpected promotion type: %s", quote.PromotionType)
	}
}
```

- [ ] **Step 2: Run the focused pricing test and verify red**

Run: `go test ./app/order/rpc/internal/pricing -run "Quote" -count=1`

Expected: FAIL because the pricing package does not exist yet.

- [ ] **Step 3: Add order-side schema for snapshot and payment order**

```sql
CREATE TABLE IF NOT EXISTS order_price_snapshot (
  id bigint NOT NULL AUTO_INCREMENT,
  order_id varchar(64) NOT NULL,
  product_id bigint NOT NULL,
  origin_price_fen bigint NOT NULL,
  final_price_fen bigint NOT NULL,
  discount_amount_fen bigint NOT NULL,
  promotion_type varchar(32) NOT NULL DEFAULT '',
  promotion_id bigint NOT NULL DEFAULT 0,
  pricing_version bigint NOT NULL DEFAULT 1,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_product (order_id, product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS payment_order (
  id varchar(64) NOT NULL,
  order_id varchar(64) NOT NULL,
  user_id bigint NOT NULL,
  amount_fen bigint NOT NULL,
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-init 1-success 2-failed 3-closed',
  channel varchar(32) NOT NULL DEFAULT 'mock',
  out_trade_no varchar(64) NOT NULL,
  paid_at timestamp NULL DEFAULT NULL,
  callback_payload json DEFAULT NULL,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_id (order_id),
  UNIQUE KEY uniq_out_trade_no (out_trade_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 4: Extend create-order request and response path**

```proto
message CreateOrderReq {
  string order_id = 1;
  int64 user_id = 2;
  int64 product_id = 3;
  int64 amount = 4;
  string request_id = 5;
  int64 expected_price_fen = 6;
}
```

```go
type CreateOrderResp struct {
	OrderId       string `json:"order_id"`
	Status        string `json:"status"`
	PayableAmount int64  `json:"payable_amount_fen"`
	PaymentOrder  string `json:"payment_order_id"`
}
```

- [ ] **Step 5: Implement quote building and order persistence in the saga branch**

```go
card, err := l.svcCtx.ProductRpc.GetProductCard(l.ctx, &product.GetProductCardReq{
	ProductId: in.ProductId,
})
if err != nil {
	return nil, status.Error(codes.Internal, "load product card failed")
}
quote := pricing.BuildQuote(pricing.PricingInput{
	ProductID:       card.ProductId,
	ProductName:     card.Name,
	Quantity:        in.Amount,
	OriginUnitPrice: card.OriginPriceFen,
	LimitedPriceFen: card.FinalPriceFen,
})
if in.ExpectedPriceFen > 0 && in.ExpectedPriceFen != quote.PayableAmountFen {
	return nil, status.Error(codes.FailedPrecondition, "price changed, please retry checkout")
}
_, err = tx.Exec(`
INSERT INTO orders (id, request_id, user_id, product_id, amount, status)
VALUES (?, ?, ?, ?, ?, 0)`,
	in.OrderId, requestID, in.UserId, in.ProductId, in.Amount,
)
_, err = tx.Exec(`
INSERT INTO order_price_snapshot (order_id, product_id, origin_price_fen, final_price_fen, discount_amount_fen, promotion_type, promotion_id, pricing_version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	in.OrderId, in.ProductId, quote.OriginAmountFen, quote.PayableAmountFen, quote.DiscountAmountFen, quote.PromotionType, 0, 1,
)
_, err = tx.Exec(`
INSERT INTO payment_order (id, order_id, user_id, amount_fen, status, channel, out_trade_no)
VALUES (?, ?, ?, ?, 0, 'mock', ?)`,
	"pay:"+in.OrderId, in.OrderId, in.UserId, quote.PayableAmountFen, "mock-"+in.OrderId,
)
```

- [ ] **Step 6: Verify create-order returns snapshot-backed payment info**

Run: `go test ./app/order/rpc/internal/pricing ./app/order/rpc/internal/logic ./app/order/api/internal/logic -run "Quote|CreateOrder" -count=1`

Expected: PASS

- [ ] **Step 7: Commit the persistence slice**

```powershell
git add app/order/rpc/internal/pricing app/order/rpc/order.proto app/order/api/internal/types/types.go app/order/api/internal/logic/createorderlogic.go app/order/api/internal/logic/createorderlogic_test.go app/order/rpc/internal/logic/createOrderLogic.go app/order/rpc/internal/logic/createOrderLogic_test.go app/order/rpc/internal/server/orderServer.go scripts/k8s/init-db.sql
git commit -m "feat: persist pricing snapshot and payment order"
```

### Task 3: Add Payment Success Callback And Order Detail Read Path

**Files:**
- Modify: `app/order/rpc/order.proto`
- Regenerate: `app/order/rpc/order/*.pb.go`
- Regenerate: `app/order/rpc/orderclient/order.go`
- Create: `app/order/rpc/internal/logic/markorderpaidlogic.go`
- Create: `app/order/rpc/internal/logic/markorderpaidlogic_test.go`
- Create: `app/order/rpc/internal/logic/getorderdetaillogic.go`
- Create: `app/order/rpc/internal/logic/getorderdetaillogic_test.go`
- Modify: `app/order/rpc/internal/server/orderServer.go`
- Create: `app/order/api/internal/handler/payorderhandler.go`
- Create: `app/order/api/internal/handler/payorderhandler_test.go`
- Create: `app/order/api/internal/handler/orderdetailhandler.go`
- Create: `app/order/api/internal/handler/orderdetailhandler_test.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/web/shop.html`

- [ ] **Step 1: Write the failing payment callback idempotency test**

```go
func TestMarkOrderPaidLogic_MarkPaid_IsIdempotent(t *testing.T) {
	svcCtx := newPaidOrderTestContext(t)
	l := NewMarkOrderPaidLogic(context.Background(), svcCtx)

	first, err := l.MarkPaid(&order.MarkOrderPaidReq{
		OrderId:       "o-paid-1",
		PaymentOrderId: "pay:o-paid-1",
		OutTradeNo:    "mock-o-paid-1",
		CallbackBody:  `{"trade_status":"SUCCESS"}`,
	})
	if err != nil || !first.Updated {
		t.Fatalf("first callback should update, resp=%#v err=%v", first, err)
	}

	second, err := l.MarkPaid(&order.MarkOrderPaidReq{
		OrderId:       "o-paid-1",
		PaymentOrderId: "pay:o-paid-1",
		OutTradeNo:    "mock-o-paid-1",
		CallbackBody:  `{"trade_status":"SUCCESS"}`,
	})
	if err != nil || second.Updated {
		t.Fatalf("second callback should be idempotent, resp=%#v err=%v", second, err)
	}
}
```

- [ ] **Step 2: Add order RPC methods for mark-paid and detail read**

```proto
message MarkOrderPaidReq {
  string order_id = 1;
  string payment_order_id = 2;
  string out_trade_no = 3;
  string callback_body = 4;
}

message MarkOrderPaidResp {
  bool updated = 1;
  string order_status = 2;
}

message GetOrderDetailReq {
  string order_id = 1;
}

message GetOrderDetailResp {
  string order_id = 1;
  int64 user_id = 2;
  string order_status = 3;
  int64 payable_amount_fen = 4;
  string payment_status = 5;
  int64 origin_price_fen = 6;
  int64 discount_amount_fen = 7;
}
```

- [ ] **Step 3: Implement conditional pay transition and callback idempotency**

```go
result, err := tx.Exec(`
UPDATE orders
SET status = 1, update_time = NOW()
WHERE id = ? AND status = 0`,
	in.OrderId,
)
rows, _ := result.RowsAffected()
if rows == 1 {
	_, err = tx.Exec(`
UPDATE payment_order
SET status = 1, paid_at = NOW(), callback_payload = CAST(? AS JSON)
WHERE id = ? AND status = 0`,
		in.CallbackBody, in.PaymentOrderId,
	)
	return &order.MarkOrderPaidResp{Updated: true, OrderStatus: "PAID"}, err
}
var currentStatus int64
if err := tx.QueryRow("SELECT status FROM orders WHERE id = ?", in.OrderId).Scan(&currentStatus); err != nil {
	return nil, err
}
return &order.MarkOrderPaidResp{Updated: false, OrderStatus: strconv.FormatInt(currentStatus, 10)}, nil
```

- [ ] **Step 4: Expose payment and order detail through order-api**

```go
func PayOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PayOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.OrderRpc.MarkOrderPaid(r.Context(), &orderrpc.MarkOrderPaidReq{
			OrderId:        req.OrderId,
			PaymentOrderId: req.PaymentOrderId,
			OutTradeNo:     req.OutTradeNo,
			CallbackBody:   `{"trade_status":"SUCCESS","source":"mock"}`,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
```

- [ ] **Step 5: Update storefront actions for order detail and pay-now**

```javascript
async function payLatestOrder(orderId, paymentOrderId, outTradeNo){
  const response = await authed("/api/order/pay", {
    method:"POST",
    jsonBody:{order_id:orderId,payment_order_id:paymentOrderId,out_trade_no:outTradeNo}
  });
  if(response.ok){
    await loadOrderDetail(orderId);
    log(`支付成功 order=${orderId}`);
  }
}
```

- [ ] **Step 6: Run the narrow verification**

Run: `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|GetOrderDetail|PayOrderHandler|OrderDetailHandler" -count=1`

Expected: PASS

- [ ] **Step 7: Commit the payment slice**

```powershell
git add app/order/rpc/order.proto app/order/rpc/internal/logic/markorderpaidlogic.go app/order/rpc/internal/logic/markorderpaidlogic_test.go app/order/rpc/internal/logic/getorderdetaillogic.go app/order/rpc/internal/logic/getorderdetaillogic_test.go app/order/rpc/internal/server/orderServer.go app/order/api/internal/handler/payorderhandler.go app/order/api/internal/handler/payorderhandler_test.go app/order/api/internal/handler/orderdetailhandler.go app/order/api/internal/handler/orderdetailhandler_test.go app/order/api/internal/handler/routes.go app/order/api/internal/handler/web/shop.html
git commit -m "feat: add payment callback and order detail flow"
```

### Task 4: Harden Timeout Close, Stock Release, And Pay-Close Race

**Files:**
- Modify: `app/order/api/job/closeorder.go`
- Create: `app/order/api/job/closeorder_test.go`
- Modify: `app/product/rpc/internal/logic/revertstocklogic.go`
- Create: `app/product/rpc/internal/logic/revertstocklogic_test.go`
- Modify: `app/order/rpc/internal/job/outbox_publisher.go`
- Modify: `scripts/k8s/init-db.sql`

- [ ] **Step 1: Write the failing race test for timeout close**

```go
func TestCloseOrderJob_HandleCloseOrder_SkipsPaidOrder(t *testing.T) {
	svcCtx := newCloseOrderTestContext(t)
	seedOrderStatus(t, svcCtx.OrderModel, "o-paid-race", 1)

	job := NewCloseOrderJob(svcCtx)
	err := job.handleCloseOrder("o-paid-race")
	if err != nil {
		t.Fatalf("paid order should be skipped, got %v", err)
	}
	assertNoRevertCall(t, svcCtx)
}
```

- [ ] **Step 2: Add reservation and close bookkeeping fields**

```sql
CREATE TABLE IF NOT EXISTS stock_reservation (
  id bigint NOT NULL AUTO_INCREMENT,
  order_id varchar(64) NOT NULL,
  product_id bigint NOT NULL,
  quantity int NOT NULL,
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-reserved 1-released 2-consumed',
  reserved_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  released_at timestamp NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_product (order_id, product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 3: Make timeout close a conditional state transition**

```go
result, err := j.svcCtx.SqlConn.ExecCtx(j.ctx, `
UPDATE orders
SET status = 2, update_time = NOW()
WHERE id = ? AND status = 0`, orderId)
if err != nil {
	return err
}
rows, _ := result.RowsAffected()
if rows == 0 {
	j.Infof("order already moved by another path, skip close: %s", orderId)
	return nil
}
```

- [ ] **Step 4: Release stock reservation only after close wins**

```go
_, err = j.svcCtx.ProductRpc.RevertStock(j.ctx, &product.RevertStockReq{
	Id:      order.ProductId,
	Num:     order.Amount,
	OrderId: orderId,
})
if err != nil {
	return err
}
_, err = j.svcCtx.SqlConn.ExecCtx(j.ctx, `
UPDATE stock_reservation
SET status = 1, released_at = NOW()
WHERE order_id = ? AND status = 0`, orderId)
```

- [ ] **Step 5: Emit close event through outbox and verify compensation tests**

Run: `go test ./app/order/api/job ./app/product/rpc/internal/logic -run "CloseOrder|RevertStock" -count=1`

Expected: PASS

- [ ] **Step 6: Commit the compensation slice**

```powershell
git add app/order/api/job/closeorder.go app/order/api/job/closeorder_test.go app/product/rpc/internal/logic/revertstocklogic.go app/product/rpc/internal/logic/revertstocklogic_test.go app/order/rpc/internal/job/outbox_publisher.go scripts/k8s/init-db.sql
git commit -m "feat: harden timeout close and stock release"
```

### Task 5: Demo Surface, Local Verification, And Documentation

**Files:**
- Modify: `app/order/api/internal/handler/web/shop.html`
- Create: `docs/TRADING_LOOP_V2_20260416.md`

- [ ] **Step 1: Add storefront anchors for transaction demo**

```html
<section class="console-summary">
  <div>最新订单：<strong id="latest-order-status">暂无</strong></div>
  <div>支付状态：<strong id="latest-payment-status">暂无</strong></div>
  <div>成交快照：<strong id="latest-price-snapshot">暂无</strong></div>
</section>
```

- [ ] **Step 2: Document the five demo flows**

```md
## Demo Flow
1. Browse `/shop` and confirm origin price plus promotion price are visible.
2. Create an order and verify `PENDING_PAYMENT`.
3. Trigger mock payment and verify the order becomes `PAID`.
4. Replay the same payment callback and verify no duplicate transition occurs.
5. Create another order, do not pay, wait for close, and verify stock is released.
```

- [ ] **Step 3: Run package verification**

Run: `go test ./app/product/rpc/... ./app/order/rpc/... ./app/order/api/... -count=1`

Expected: PASS

- [ ] **Step 4: Run local integration verification**

Run: `powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1`

Expected:
- `product-rpc` listens on `8080`
- `order-rpc` listens on `8090`
- `order-api` listens on `8888`

Run:

```powershell
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8888/shop
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8888/api/shop/catalog
```

Expected:
- `/shop` returns `200`
- `/api/shop/catalog` returns `200`

- [ ] **Step 5: Stop local services and commit docs**

Run: `powershell -ExecutionPolicy Bypass -File scripts/local/stop-all.ps1 -WithDeps`

Expected: no leftover listeners on `8888`, `8890`, `8080`, `8090`

Run:

```powershell
git add app/order/api/internal/handler/web/shop.html docs/TRADING_LOOP_V2_20260416.md
git commit -m "docs: add trading loop v2 demo guide"
```

## Acceptance Checklist

- storefront product cards show origin price, effective price, stock summary, and promotion tag
- create-order recalculates price instead of trusting display-only values
- order creation persists price snapshot and payment order
- order enters `PENDING_PAYMENT` after successful create path
- payment callback moves `PENDING_PAYMENT` orders to `PAID`
- repeated payment callback is idempotent
- timeout close only closes unpaid orders
- stock reservation is released on timeout close
- pay-close race ends with one stable final state
- `go test ./app/product/rpc/... ./app/order/rpc/... ./app/order/api/... -count=1` passes

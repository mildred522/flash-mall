# Trading Loop V2 Design

## Goal

Build the next interview-grade slice for `flash-mall` by extending the existing order flow into a full transaction loop: dynamic pricing, price snapshot, stock reservation, payment success callback, timeout close, and stock release.

## Positioning

This version is not a full supply-chain platform. The primary target is the customer-facing transaction path:

- product display
- promotion pricing
- order creation
- stock reservation
- payment success
- timeout close

Supply-chain concepts only enter as minimal support for replenishment and product-supplier relation.

## Capability Boundary

### In Scope

- storefront product cards show origin price, effective price, inventory hint, and promotion tag
- checkout recalculates price instead of trusting display price
- order creation stores price snapshot and enters `PENDING_PAYMENT`
- stock is reserved before payment and released on timeout close
- payment order is created together with the order
- payment success callback moves order to `PAID`
- repeated payment callback is idempotent
- timeout close only works on unpaid orders
- transaction events can continue to use the existing SAGA and outbox infrastructure

### Out of Scope

- real third-party payment integration
- refund flow
- coupon center or combinational discount engine
- full procurement workflow
- supplier settlement
- merchant multi-tenancy

## Service Responsibility

- `order-api`: BFF and storefront entry only
- `order-rpc`: transaction orchestration, order state machine, payment order, price snapshot, timeout close
- `product-rpc`: product base info, promotion-facing product fields, stock reservation and release
- `auth-service`: identity, session, and account security only

The system should continue to present one browser-facing entry, while keeping transaction truth in `order-rpc` and product or stock truth in `product-rpc`.

## Core Flow

### 1. Browse

The storefront queries product data and receives:

- product basic info
- origin price
- current effective promotion result
- stock summary

This is display-time pricing only.

### 2. Create Order

When the user places an order:

1. validate authenticated user context
2. recalculate price using current pricing rules
3. reserve stock
4. create order, order item, price snapshot, and payment order
5. set order status to `PENDING_PAYMENT`

### 3. Payment Success

Payment success arrives through a simulated callback path. The callback:

- checks payment order state
- checks order state
- transitions `PENDING_PAYMENT -> PAID` exactly once
- ignores repeated success callbacks after the first success

### 4. Timeout Close

An unpaid order can be closed by timeout worker or delayed event:

- only `PENDING_PAYMENT` orders may become `CLOSED`
- close releases reserved stock
- paid orders must be skipped even if the timeout worker arrives later

## State Model

### Order Status

- `PENDING_PAYMENT`
- `PAID`
- `CLOSED`

### Payment Order Status

- `INIT`
- `SUCCESS`
- `FAILED`
- `CLOSED`

The order and payment order remain separate so payment retry and order lifecycle do not collapse into one field.

## Data Model

### Product

- `id`
- `name`
- `status`
- `origin_price`
- `sale_price`
- `supplier_id`

### PromotionRule

- `id`
- `product_id`
- `type`
- `discount_value`
- `threshold_amount`
- `starts_at`
- `ends_at`
- `status`

First supported rule types:

- `LIMITED_PRICE`
- `FULL_REDUCTION`

### Order

- `id`
- `user_id`
- `status`
- `total_amount`
- `payable_amount`
- `discount_amount`
- `request_id`
- `expires_at`
- `paid_at`
- `closed_at`

### OrderItem

- `order_id`
- `product_id`
- `product_name`
- `quantity`
- `origin_unit_price`
- `final_unit_price`
- `discount_amount`

### OrderPriceSnapshot

- `order_id`
- `product_id`
- `origin_price`
- `promotion_type`
- `promotion_id`
- `discount_amount`
- `final_price`
- `pricing_version`

### PaymentOrder

- `id`
- `order_id`
- `user_id`
- `amount`
- `status`
- `channel`
- `out_trade_no`
- `paid_at`
- `callback_payload`

### StockReservation

- `order_id`
- `product_id`
- `quantity`
- `status`
- `reserved_at`
- `released_at`

### Minimal Supply-Side Support

- `Supplier`
- `ProductSupplyRelation`

These are support models only, not a separate procurement product.

## Pricing Rules

V2 keeps pricing intentionally narrow:

1. start from product origin price
2. apply one active `LIMITED_PRICE` rule if matched
3. apply one order-level `FULL_REDUCTION` result if matched
4. persist final numbers into snapshot

No coupon stacking, member price stacking, or multi-rule conflict engine in this version.

## Consistency Strategy

### Price Consistency

Display price is not transaction truth. Checkout recalculates price and stores snapshot so later product or promotion changes do not rewrite historical orders.

### Stock Consistency

Stock is reserved at order creation time, not after payment success. Timeout close releases reservation.

### Create-Order Idempotency

Existing `request_id` idempotency remains the guard:

- repeated request returns the same order result
- repeated request must not reserve stock again
- repeated request must not create another payment order

### Payment Callback Idempotency

Repeated callbacks are expected. The callback handler must:

- return success if payment order is already `SUCCESS`
- return success if order is already `PAID`
- perform state transition only once

### Pay-vs-Close Race

Both payment success and timeout close compete on the same order. The only allowed transitions are:

- `PENDING_PAYMENT -> PAID`
- `PENDING_PAYMENT -> CLOSED`

Both transitions must be condition-based updates. Whoever wins the state transition owns the final result. The loser sees a non-matching current state and exits without side effects.

### Event Consistency

The existing outbox pattern should continue to publish transaction events such as:

- `order.created`
- `payment.succeeded`
- `order.closed`
- `stock.released`

This keeps local transaction and event publication aligned.

## Demo Flows

1. normal purchase: browse, create order, pay successfully, observe `PAID`
2. price snapshot: change current product price after order creation and verify historical order price stays unchanged
3. timeout close: create order, do not pay, observe `CLOSED` and stock release
4. payment callback idempotency: call success callback repeatedly and verify only one effective transition
5. pay-close race: simulate near-timeout payment and verify final state is either `PAID` or `CLOSED`, never both

## Interview Narrative

This phase upgrades the project from “can create orders” to “has a business-grade transaction loop”. The strongest talking points are:

- dynamic pricing plus snapshot instead of trusting current product price
- reservation-based stock control instead of post-payment stock deduction
- explicit payment order plus idempotent callback handling
- race-safe state transitions between payment success and timeout close
- continued use of SAGA and outbox for distributed consistency

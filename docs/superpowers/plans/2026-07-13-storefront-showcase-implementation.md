# 商家店铺与首页商品橱窗 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为每个审核通过的商家提供统一模板店铺，让商家决定哪些商品在自己的店铺上架，让管理员用 12 个固定首页橱窗位人工发布跨店商品，并提供可解释、只建议不自动发布的均衡推荐。

**Architecture:** 保留现有 Hertz 作为统一 HTTP/BFF 入口、Go-zero product-rpc 作为商品读路径、Kitex 作为结算前库存写命令。店铺资料位于 `mall_order` 的商家领域，首页橱窗位于 `mall_product` 的商品展示领域；公开列表使用商品卡片快照与库存快照，不为首页增加 Kitex 调用。管理员发布采用完整布局、乐观锁和事务替换，公开端只隐藏失效槽位，后台保留失效原因和槽位位置。

**Tech Stack:** Go 1.24、CloudWeGo Hertz、Go-zero RPC、Kitex、MySQL 5.7（当前 Compose 运行版本）、Prometheus、React 19、TypeScript 5.8、Ant Design 5、Vitest、Docker Compose、Chrome。

---

## 实施边界与不变量

- 商家店铺采用统一模板，只允许编辑 Logo、横幅和简介；不做页面装修器。
- 商品 `status=1` 代表商家在自己店铺上架，`status=2` 代表下架；商家不能直接决定首页展示。
- 首页固定 12 个槽位，同一商品不能重复，同一商家最多占 2 个槽位。
- 失效商品不会自动补位：公开首页隐藏它，管理员后台保留槽位与失效原因。
- 推荐接口只返回候选和解释，不写数据库、不自动发布。
- 首页、店铺页和详情页继续使用 Go-zero product-rpc、`product_card_snapshot`、`product_stock_snapshot`；下单预占、释放、确认仍以 Kitex 为准。
- `/products/*any` 已用于内置商品图片，因此页面路由固定为单数 `/product/{id}` 和 `/store/{merchant_id}`。
- 旧 Go-zero entry-api 只作为对照实现保留；本计划只改 Hertz 新链路，不把新接口反向写进旧 entry-api。
- 现有 `CatalogProductIDs` 只从 Hertz 配置和 Hertz 读取逻辑移除；旧 entry-api 配置保留，便于演示迁移前后的架构差异。

## API 契约

```text
GET  /api/shop/products/detail?product_id={product_id}
GET  /api/shop/stores/detail?merchant_id={merchant_id}
GET  /api/shop/stores/products?merchant_id={merchant_id}&page=1&page_size=20
GET  /api/shop/catalog

GET  /api/merchant/store/profile
POST /api/merchant/store/profile
POST /api/merchant/store/assets

GET  /api/admin/showcase
GET  /api/admin/showcase/candidates?page=1&page_size=20&keyword=
POST /api/admin/showcase/publish
```

店铺资料更新请求固定使用乐观锁：

```json
{
  "logo_url": "/uploads/stores/1000/logo-abcd.webp",
  "banner_url": "/uploads/stores/1000/banner-efgh.webp",
  "description": "Flash Mall 自营店",
  "expected_version": 3
}
```

橱窗发布语义是“用本次请求完整替换旧布局”。请求只提交非空槽位，缺失的 1～12 位置视为空槽：

```json
{
  "expected_version": 7,
  "items": [
    {"slot_no": 1, "product_id": 100},
    {"slot_no": 2, "product_id": 101}
  ]
}
```

## 文件结构

### 后端新增

- `app/gateway/hertz/internal/handler/storefront_schema.go`：幂等创建店铺、橱窗表和商品创建时间迁移。
- `app/gateway/hertz/internal/handler/storefront_schema_test.go`：验证迁移顺序、旧商品回填和种子不覆盖人工布局。
- `app/gateway/hertz/internal/handler/merchant_store.go`：商家本人读取、更新店铺资料。
- `app/gateway/hertz/internal/handler/merchant_store_test.go`：资料权限、乐观锁和字段校验。
- `app/gateway/hertz/internal/handler/store_asset.go`：商家店铺 Logo/横幅上传和静态读取。
- `app/gateway/hertz/internal/handler/store_asset_test.go`：文件类型、大小、路径隔离和权限。
- `app/gateway/hertz/internal/handler/storefront.go`：公开店铺详情、店内商品和商品所属店铺信息。
- `app/gateway/hertz/internal/handler/storefront_test.go`：公开店铺可见性和分页。
- `app/gateway/hertz/internal/handler/showcase.go`：公开橱窗读取、后台布局读取和事务发布。
- `app/gateway/hertz/internal/handler/showcase_test.go`：失效隐藏、槽位保留、版本冲突和约束校验。
- `app/gateway/hertz/internal/handler/showcase_recommendation.go`：候选查询、确定性打分与解释。
- `app/gateway/hertz/internal/handler/showcase_recommendation_test.go`：评分边界、商家配额和稳定排序。
- `app/gateway/hertz/internal/handler/showcase_metrics.go`：读取、发布、失效槽位和推荐延迟指标。
- `app/gateway/hertz/internal/handler/metrics_handler.go`：以 Prometheus 文本格式暴露 Hertz 默认 registry。

### 后端修改

- `scripts/k8s/init-db.sql`：正式数据库结构、迁移、默认店铺资料和 100–104 号商品橱窗种子。
- `app/product/rpc/desc/product.sql`：商品表补 `create_time`，保持开发 schema 一致。
- `app/gateway/hertz/internal/config/config.go`：移除 Hertz 的 `CatalogProductIDs`。
- `app/gateway/hertz/etc/gateway.yaml`：移除 Hertz 橱窗硬编码列表。
- `deploy/config/gateway.yaml`：移除 Compose 中 Hertz 橱窗硬编码列表。
- `app/gateway/hertz/internal/handler/catalog.go`：公开首页改读数据库橱窗。
- `app/gateway/hertz/internal/handler/types.go`：增加店铺、橱窗和候选响应类型，扩展商品卡片商家信息。
- `app/gateway/hertz/internal/handler/routes.go`：注册公开、商家和管理员新路由以及页面/静态资源路由。
- `app/gateway/hertz/internal/handler/routes_test.go`：断言路由、方法和鉴权中间件。
- `app/gateway/hertz/internal/handler/admin_audit.go`：增加橱窗发布审计事件。
- `app/gateway/hertz/internal/handler/migration_status.go`：将新 Hertz 路由纳入迁移状态输出。

### 前端新增

- `frontend/packages/shop/src/pages/ProductDetailPage.tsx`：商品详情和进入店铺入口。
- `frontend/packages/shop/src/pages/ProductDetailPage.test.tsx`：详情请求、店铺导航和购买。
- `frontend/packages/shop/src/pages/StorePage.tsx`：统一店铺模板和店内商品分页。
- `frontend/packages/shop/src/pages/StorePage.test.tsx`：店铺资料、空态、分页和下架过滤。
- `frontend/packages/shop/src/App.test.tsx`：公开页面路径分流。
- `frontend/packages/merchant/src/pages/StoreSettingsPage.tsx`：商家店铺资料编辑和图片上传。
- `frontend/packages/merchant/src/pages/StoreSettingsPage.test.tsx`：加载、上传、保存和版本冲突。
- `frontend/packages/admin/src/pages/ShowcasePage.tsx`：12 槽布局、候选推荐、拖放/键盘排序和发布。
- `frontend/packages/admin/src/pages/ShowcasePage.test.tsx`：添加、移除、排序、失效标记和发布冲突。

### 前端修改

- `frontend/packages/shared/src/types.ts`：共享店铺、橱窗、候选、商品商家字段。
- `frontend/packages/shared/src/product-image-upload.ts`：抽取通用图片上传函数并保留商品上传兼容包装。
- `frontend/packages/shared/src/product-image-upload.test.ts`：覆盖自定义上传地址。
- `frontend/packages/shared/src/index.ts`：导出新增类型和上传函数。
- `frontend/packages/shop/src/App.tsx`：解析 `/product/{id}`、`/store/{id}` 和浏览器前进后退。
- `frontend/packages/shop/src/components/ProductCard.tsx`：卡片主体进详情、店铺链接、独立购买按钮。
- `frontend/packages/shop/src/components/ProductCard.test.tsx`：阻止店铺链接误触购买并验证三个动作。
- `frontend/packages/shop/src/pages/HomePage.tsx`：消费橱窗顺序和失效过滤后的响应。
- `frontend/packages/shop/src/styles/shop.css`：详情、店铺横幅、店铺链接和响应式样式。
- `frontend/packages/merchant/src/App.tsx`：增加“店铺设置”菜单和路由。
- `frontend/packages/merchant/src/App.test.tsx`：断言菜单与页面映射。
- `frontend/packages/admin/src/App.tsx`：增加“首页橱窗”菜单和路由。
- `frontend/packages/admin/src/App.test.tsx`：断言菜单与页面映射。
- `app/entry/api/internal/handler/web/shop.html`：由前端构建生成。
- `app/entry/api/internal/handler/web/merchant.html`：由前端构建生成。
- `app/entry/api/internal/handler/web/admin.html`：由前端构建生成。

---

### Task 1: 建立幂等数据库结构和迁移入口

**Files:**
- Create: `app/gateway/hertz/internal/handler/storefront_schema.go`
- Create: `app/gateway/hertz/internal/handler/storefront_schema_test.go`
- Modify: `scripts/k8s/init-db.sql`
- Modify: `app/product/rpc/desc/product.sql`

- [ ] **Step 1: 先写迁移行为测试**

使用 `sqlmock` 覆盖三种路径：缺少 `product.create_time` 时先加 nullable 列、按 `NOW() - INTERVAL 31 DAY` 回填、再改成非空；已有列时不执行 ALTER；默认种子只在 `homepage_showcase_item` 完全为空时写入，不覆盖现有人工布局。

测试中的关键期望固定为：

```go
mock.ExpectQuery("SELECT COUNT\\(1\\).*information_schema.COLUMNS").
	WithArgs("mall_product", "product", "create_time").
	WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
mock.ExpectExec("ALTER TABLE mall_product.product ADD COLUMN create_time").
	WillReturnResult(sqlmock.NewResult(0, 0))
mock.ExpectExec("UPDATE mall_product.product SET create_time = NOW\\(\\) - INTERVAL 31 DAY").
	WillReturnResult(sqlmock.NewResult(0, 5))
mock.ExpectExec("ALTER TABLE mall_product.product MODIFY COLUMN create_time").
	WillReturnResult(sqlmock.NewResult(0, 0))
```

- [ ] **Step 2: 运行测试并确认先失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestEnsureStorefrontSchema|TestSeedDefaultShowcase' -count=1`

Expected: 编译失败，提示 `ensureStorefrontSchema` 尚未定义。

- [ ] **Step 3: 实现单一幂等 schema 入口**

实现以下入口，所有新 handler 在首次访问前调用它；用互斥锁和 `schemaReady bool` 只缓存成功结果，失败必须释放锁并允许下次请求重试，不能用普通 `sync.Once` 吞掉首次错误：

```go
func ensureStorefrontSchema(ctx context.Context, db *sql.DB) error {
	if err := ensureMerchantStoreProfileTable(ctx, db); err != nil {
		return err
	}
	if err := ensureProductCreateTimeColumn(ctx, db); err != nil {
		return err
	}
	if err := ensureHomepageShowcaseTables(ctx, db); err != nil {
		return err
	}
	return seedDefaultShowcase(ctx, db)
}
```

正式表结构固定为：

```sql
CREATE TABLE IF NOT EXISTS mall_order.merchant_store_profile (
  merchant_id bigint NOT NULL,
  logo_url varchar(512) NOT NULL DEFAULT '',
  banner_url varchar(512) NOT NULL DEFAULT '',
  description varchar(1000) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mall_product.homepage_showcase (
  id bigint NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  operator_id bigint NOT NULL DEFAULT 0,
  publish_time timestamp NULL DEFAULT NULL,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mall_product.homepage_showcase_item (
  showcase_id bigint NOT NULL,
  slot_no tinyint NOT NULL,
  product_id bigint NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (showcase_id, slot_no),
  UNIQUE KEY uk_showcase_product (showcase_id, product_id),
  KEY ix_showcase_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

商品创建时间迁移必须使用两阶段 ALTER，避免给旧商品制造“刚上架”的推荐优势：

```sql
ALTER TABLE mall_product.product ADD COLUMN create_time datetime NULL;
UPDATE mall_product.product
SET create_time = NOW() - INTERVAL 31 DAY
WHERE create_time IS NULL;
ALTER TABLE mall_product.product
MODIFY COLUMN create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
```

种子先 `INSERT ... ON DUPLICATE KEY UPDATE id=id` 创建 `id=1` 的橱窗，再检查 item 数量；仅当数量为 0 时插入槽位 1–5 与商品 100–104，槽位 6–12 保持空。

- [ ] **Step 4: 同步正式初始化 SQL 和 product-rpc schema**

在 `scripts/k8s/init-db.sql` 使用 `information_schema` 判断并迁移 `create_time`，加入三张表、默认自营店资料和不覆盖人工布局的种子。在 `app/product/rpc/desc/product.sql` 的 `product` 表加入：

```sql
create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
KEY ix_product_create_time (create_time)
```

- [ ] **Step 5: 运行聚焦测试和 SQL 静态检查**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestEnsureStorefrontSchema|TestSeedDefaultShowcase' -count=1`

Expected: PASS。

Run: `grep -nE 'merchant_store_profile|homepage_showcase|create_time' scripts/k8s/init-db.sql app/product/rpc/desc/product.sql`

Expected: 两份 schema 都含新结构，初始化脚本含旧数据回填。

- [ ] **Step 6: 提交数据库基础**

```bash
git add app/gateway/hertz/internal/handler/storefront_schema.go \
  app/gateway/hertz/internal/handler/storefront_schema_test.go \
  scripts/k8s/init-db.sql app/product/rpc/desc/product.sql
git commit -m "feat: 建立店铺与首页橱窗数据结构"
```

### Task 2: 实现商家店铺资料读写

**Files:**
- Create: `app/gateway/hertz/internal/handler/merchant_store.go`
- Create: `app/gateway/hertz/internal/handler/merchant_store_test.go`
- Modify: `app/gateway/hertz/internal/handler/types.go`

- [ ] **Step 1: 定义类型和失败测试**

在 `types.go` 增加：

```go
type MerchantStoreProfile struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	LogoURL      string `json:"logo_url"`
	BannerURL    string `json:"banner_url"`
	Description  string `json:"description"`
	Version      int64  `json:"version"`
}

type merchantStoreUpdateReq struct {
	LogoURL         string `json:"logo_url"`
	BannerURL       string `json:"banner_url"`
	Description     string `json:"description"`
	ExpectedVersion int64  `json:"expected_version"`
}
```

测试四条规则：`selectedMerchantID` 只能读取自己的店铺；空资料返回商家名和 `version=0`；简介超过 1000 字符返回 400；并发版本不一致返回 409 且不覆盖新数据。

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestMerchantStore(Profile|Update)' -count=1`

Expected: 编译失败，handler 尚未定义。

- [ ] **Step 3: 实现读取与乐观锁更新**

查询必须以中间件选中的 `merchant_id` 为准，不接受请求体传入商家 ID：

```sql
SELECT m.id, m.name,
       COALESCE(p.logo_url, ''), COALESCE(p.banner_url, ''),
       COALESCE(p.description, ''), COALESCE(p.version, 0)
FROM mall_order.merchant m
LEFT JOIN mall_order.merchant_store_profile p ON p.merchant_id = m.id
WHERE m.id = ? AND m.status = 1
```

更新放在事务内。`expected_version=0` 只允许首次 `INSERT`；大于 0 时执行：

```sql
UPDATE mall_order.merchant_store_profile
SET logo_url=?, banner_url=?, description=?, version=version+1
WHERE merchant_id=? AND version=?
```

`RowsAffected()==0` 返回 HTTP 409，成功后重新查询并返回最新资料。URL 允许空字符串、`/uploads/stores/`、站内 `/products/` 或合法 HTTP(S) 地址；拒绝 `javascript:`、`data:`、包含用户信息的 URL 和非 HTTP(S) scheme。

- [ ] **Step 4: 运行测试**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestMerchantStore(Profile|Update)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交店铺资料后端**

```bash
git add app/gateway/hertz/internal/handler/merchant_store.go \
  app/gateway/hertz/internal/handler/merchant_store_test.go \
  app/gateway/hertz/internal/handler/types.go
git commit -m "feat: 支持商家维护店铺资料"
```

### Task 3: 实现店铺图片上传和隔离读取

**Files:**
- Create: `app/gateway/hertz/internal/handler/store_asset.go`
- Create: `app/gateway/hertz/internal/handler/store_asset_test.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`

- [ ] **Step 1: 写上传安全测试和路由测试**

覆盖 JPEG、PNG、WebP 成功，伪造扩展名、SVG、超过 5 MiB、未登录、非商家失败；断言返回 URL 包含当前商家目录，而不是客户端提交的目录：

```text
/uploads/stores/{selected_merchant_id}/{asset_type}-{crypto-random-16-byte-hex}.{ext}
```

`asset_type` 只接受 `logo` 或 `banner`。

- [ ] **Step 2: 确认测试失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestMerchantStoreAsset|TestRoutes_Store' -count=1`

Expected: 新 handler 和路由尚不存在。

- [ ] **Step 3: 实现上传与静态读取**

复用 `admin_product_image.go` 已有的 MIME 嗅探、扩展名和原子落盘规则，但把公共文件写入辅助函数：

```go
func saveUploadedImage(fileHeader *multipart.FileHeader, root, relativeDir, prefix string) (string, error)
```

上传流程必须先从中间件上下文取 `selectedMerchantID`，再生成目录；使用 `filepath.Clean` 后验证目标绝对路径仍位于配置的上传根目录内。文件名使用 `crypto/rand` 生成 16 字节随机值，避免可预测覆盖。静态读取沿用产品图片 handler 的路径穿越防护和缓存头。

注册：

```go
h.GET("/uploads/stores/*any", StoreUploadStaticHandler(svcCtx))
h.POST("/api/merchant/store/assets",
	middleware.RequireMerchant(svcCtx.Config.JwtAuthSecret),
	MerchantStoreAssetUploadHandler(svcCtx))
```

- [ ] **Step 4: 运行测试**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestMerchantStoreAsset|TestRoutes_Store' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交店铺资源链路**

```bash
git add app/gateway/hertz/internal/handler/store_asset.go \
  app/gateway/hertz/internal/handler/store_asset_test.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go \
  app/gateway/hertz/internal/handler/admin_product_image.go
git commit -m "feat: 支持商家上传店铺图片"
```

### Task 4: 建立公开店铺和商品到店铺的导航数据

**Files:**
- Create: `app/gateway/hertz/internal/handler/storefront.go`
- Create: `app/gateway/hertz/internal/handler/storefront_test.go`
- Modify: `app/gateway/hertz/internal/handler/types.go`
- Modify: `app/gateway/hertz/internal/handler/catalog.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`

- [ ] **Step 1: 写公开可见性测试**

覆盖：启用商家可查看；停用/不存在商家返回 404；店内只出现 `product.status=1`；库存为 0 的商品仍可在店铺页展示并标记售罄；详情响应包含所属商家 ID、名称、Logo 和店铺 URL。

商品卡片扩展为：

```go
type ProductCard struct {
	// 保留现有字段
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	MerchantLogo string `json:"merchant_logo"`
	StoreURL     string `json:"store_url"`
	StoreStatus  int64  `json:"store_status"`
	SlotNo       int    `json:"slot_no,omitempty"`
}
```

- [ ] **Step 2: 确认测试失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestStorefront|TestProductDetailIncludesMerchant' -count=1`

Expected: 缺少公开店铺 handler 或响应字段。

- [ ] **Step 3: 实现店铺详情和店内商品查询**

店铺详情从 `mall_order.merchant` LEFT JOIN 资料表读取。店内商品 ID 从 `mall_product.product` 读取，固定条件 `merchant_id=? AND status=1`，支持商品名称 keyword，排序 `create_time DESC, id DESC`，分页上限 100；再批量调用一次 product-rpc 补齐卡片，不允许每件商品单独 RPC。商品详情额外批量读取最多 4 件同店启用商品，排除当前商品，不执行个性化推荐。

库存展示继续使用 `product_stock_snapshot`；没有快照时可回退 product-rpc 的 stock，但不得调用 Kitex。

注册公开路由和页面回退：

```go
h.GET("/product/*any", StaticPageHandler("shop.html"))
h.GET("/store/*any", StaticPageHandler("shop.html"))
h.GET("/api/shop/stores/detail", StoreDetailHandler(svcCtx))
h.GET("/api/shop/stores/products", StoreProductListHandler(svcCtx))
```

- [ ] **Step 4: 运行测试**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestStorefront|TestProductDetailIncludesMerchant|TestRoutes_Store' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交公开店铺 API**

```bash
git add app/gateway/hertz/internal/handler/storefront.go \
  app/gateway/hertz/internal/handler/storefront_test.go \
  app/gateway/hertz/internal/handler/types.go \
  app/gateway/hertz/internal/handler/catalog.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go
git commit -m "feat: 提供公开店铺与商品归属信息"
```

### Task 5: 将首页目录切换为数据库橱窗

**Files:**
- Create: `app/gateway/hertz/internal/handler/showcase.go`
- Create: `app/gateway/hertz/internal/handler/showcase_test.go`
- Modify: `app/gateway/hertz/internal/handler/catalog.go`
- Modify: `app/gateway/hertz/internal/handler/types.go`
- Modify: `app/gateway/hertz/internal/config/config.go`
- Modify: `app/gateway/hertz/etc/gateway.yaml`
- Modify: `deploy/config/gateway.yaml`

- [ ] **Step 1: 写公开与后台读取测试**

同一份 12 槽数据分别验证两个视图：

- 公开 `/api/shop/catalog`：按 `slot_no` 排序，隐藏商品不存在、商品下架、商家不存在/停用、库存为 0 的槽位，`total` 是实际可见数而不是固定 12。
- 后台 `/api/admin/showcase`：返回 operator、publish_time 和全部 12 槽；有效槽 `valid=true`，失效槽 `valid=false` 并分别返回 `product_not_found`、`product_inactive`、`merchant_not_found`、`merchant_inactive`、`out_of_stock`，空槽用 `empty=true` 表达且不伪造失效原因。

响应类型固定为：

```go
type ShowcaseSlot struct {
	SlotNo    int          `json:"slot_no"`
	ProductID int64       `json:"product_id"`
	Empty     bool         `json:"empty"`
	Valid     bool         `json:"valid"`
	InvalidReason string   `json:"invalid_reason,omitempty"`
	Product   *ProductCard `json:"product,omitempty"`
}

type ShowcaseResp struct {
	Version     int64          `json:"version"`
	OperatorID  int64          `json:"operator_id"`
	PublishTime string         `json:"publish_time"`
	Items       []ShowcaseSlot `json:"items"`
}
```

- [ ] **Step 2: 确认测试失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestShowcase(Read|PublicCatalog)' -count=1`

Expected: `loadShowcase` 和新响应类型尚未定义。

- [ ] **Step 3: 实现单次布局读取和批量商品补齐**

先读取 12 个槽位及商品/商家/快照状态，再只把有效 product ID 批量传给现有 `loadProductCards`。公开目录不得再读取 `svcCtx.Config.CatalogProductIDs`。后台响应必须人工补齐未落库的空槽，确保始终返回 `slot_no=1..12`。

Hertz 配置删除：

```go
CatalogProductIDs []int64
```

同时删除两份 Hertz YAML 中的 `CatalogProductIDs` 块，但保留 `deploy/config/entry-api.yaml` 的旧配置。

- [ ] **Step 4: 运行测试并确认旧目录逻辑已消失**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestShowcase(Read|PublicCatalog)' -count=1`

Expected: PASS。

Run: `grep -R -n 'CatalogProductIDs' app/gateway/hertz deploy/config/gateway.yaml || true`

Expected: 无输出。

- [ ] **Step 5: 提交首页读路径切换**

```bash
git add app/gateway/hertz/internal/handler/showcase.go \
  app/gateway/hertz/internal/handler/showcase_test.go \
  app/gateway/hertz/internal/handler/catalog.go \
  app/gateway/hertz/internal/handler/types.go \
  app/gateway/hertz/internal/config/config.go \
  app/gateway/hertz/etc/gateway.yaml deploy/config/gateway.yaml
git commit -m "feat: 使用数据库橱窗驱动首页商品"
```

### Task 6: 实现管理员完整布局发布

**Files:**
- Modify: `app/gateway/hertz/internal/handler/showcase.go`
- Modify: `app/gateway/hertz/internal/handler/showcase_test.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`
- Modify: `app/gateway/hertz/internal/handler/admin_audit.go`

- [ ] **Step 1: 写纯校验和事务测试**

建立表驱动测试覆盖：任一槽位越界、槽位重复、商品重复返回 400；商品不存在、商品下架、商家不存在/停用、库存归零和版本冲突返回 409 并带具体槽位原因；同商家 3 件返回 400；少于 12 个 item 合法且缺失槽位视为空；成功时先锁版本行、删除旧 item、批量插入请求 item、递增版本并提交。

纯校验函数签名固定为：

```go
func validateShowcaseDraft(items []showcasePublishItem, products map[int64]showcaseProductState) error
```

- [ ] **Step 2: 确认测试失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestValidateShowcaseDraft|TestAdminShowcasePublish' -count=1`

Expected: 发布函数尚未定义。

- [ ] **Step 3: 实现事务发布**

事务流程必须保持以下顺序：

```text
BEGIN
SELECT version FROM mall_product.homepage_showcase WHERE id=1 FOR UPDATE
比较 expected_version，不一致则 ROLLBACK + 409
批量查询所有非空 product_id 的商品状态和 merchant 状态
执行 validateShowcaseDraft
DELETE FROM mall_product.homepage_showcase_item WHERE showcase_id=1
按 slot_no 批量 INSERT 非空项
UPDATE homepage_showcase SET version=version+1, operator_id=?, publish_time=NOW() WHERE id=1
COMMIT
记录审计事件
```

审计事件名固定为 `homepage_showcase.publish`，详情记录旧版本、新版本和 12 槽 product ID，不记录管理员 token。

注册：

```go
h.GET("/api/admin/showcase", middleware.RequireAdmin(secret), AdminShowcaseHandler(svcCtx))
h.POST("/api/admin/showcase/publish", middleware.RequireAdmin(secret), AdminShowcasePublishHandler(svcCtx))
```

- [ ] **Step 4: 运行测试**

Run: `go test ./app/gateway/hertz/internal/handler -run 'TestValidateShowcaseDraft|TestAdminShowcasePublish|TestRoutes_Showcase' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交管理员发布 API**

```bash
git add app/gateway/hertz/internal/handler/showcase.go \
  app/gateway/hertz/internal/handler/showcase_test.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go \
  app/gateway/hertz/internal/handler/admin_audit.go
git commit -m "feat: 支持管理员事务发布首页橱窗"
```

### Task 7: 实现可解释且确定性的候选推荐

**Files:**
- Create: `app/gateway/hertz/internal/handler/showcase_recommendation.go`
- Create: `app/gateway/hertz/internal/handler/showcase_recommendation_test.go`
- Modify: `app/gateway/hertz/internal/handler/types.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`

- [ ] **Step 1: 用表驱动测试锁定评分边界**

评分必须精确匹配：

```go
func salesScore(sales7d int64) int {
	switch {
	case sales7d >= 21: return 40
	case sales7d >= 11: return 32
	case sales7d >= 6: return 24
	case sales7d >= 3: return 16
	case sales7d >= 1: return 8
	default: return 0
	}
}

func stockScore(stock int64) int {
	switch {
	case stock >= 51: return 15
	case stock >= 21: return 11
	case stock >= 6: return 7
	case stock >= 1: return 3
	default: return 0
	}
}

func freshnessScore(ageDays int) int {
	switch {
	case ageDays <= 7: return 20
	case ageDays <= 14: return 14
	case ageDays <= 30: return 7
	default: return 0
	}
}

func diversityScore(currentMerchantSlots int) int {
	if currentMerchantSlots == 0 { return 15 }
	if currentMerchantSlots == 1 { return 6 }
	return -1
}
```

另测同分排序：`sales_7d DESC`、`create_time DESC`、`product_id DESC`；当前已有 2 个槽位的商家被排除。

- [ ] **Step 2: 确认测试失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test(Recommendation|SalesScore|StockScore|FreshnessScore|DiversityScore)' -count=1`

Expected: 评分函数尚未定义。

- [ ] **Step 3: 实现候选查询、解释和分页**

支持 `page`、`page_size`、`keyword` 和 `merchant_id`；只查询启用商家的上架商品，排除已经占据橱窗的商品，并按当前**有效**槽位计算商家已有数量；销量统计来自 `mall_order.orders.amount`，条件为最近 7 天且状态在 `1,3,4,5,6`；库存优先 `mall_product.product_stock_snapshot.available_stock`；有效促销按当前时间命中加 10 分。候选结果按完整查询参数缓存 60 秒，发布成功后立即失效缓存。

响应每项必须包含可展示解释：

```go
type ShowcaseCandidate struct {
	Product       ProductCard `json:"product"`
	Score         int         `json:"score"`
	Sales7d       int64       `json:"sales_7d"`
	SalesScore    int         `json:"sales_score"`
	StockScore    int         `json:"stock_score"`
	PromotionScore int        `json:"promotion_score"`
	FreshnessScore int        `json:"freshness_score"`
	DiversityScore int        `json:"diversity_score"`
	Reasons       []string    `json:"reasons"`
}
```

`keyword` 只匹配商品名和商家名，`merchant_id` 精确筛选商家，`page_size` 最大 50；打分和排序在同一请求快照内完成，不写数据库。

注册管理员 GET 路由 `/api/admin/showcase/candidates`。

- [ ] **Step 4: 运行测试**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test(Recommendation|SalesScore|StockScore|FreshnessScore|DiversityScore|Routes_Showcase)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交推荐接口**

```bash
git add app/gateway/hertz/internal/handler/showcase_recommendation.go \
  app/gateway/hertz/internal/handler/showcase_recommendation_test.go \
  app/gateway/hertz/internal/handler/types.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go
git commit -m "feat: 提供可解释的首页商品推荐"
```

### Task 8: 补齐共享前端类型和通用图片上传

**Files:**
- Modify: `frontend/packages/shared/src/types.ts`
- Modify: `frontend/packages/shared/src/product-image-upload.ts`
- Modify: `frontend/packages/shared/src/product-image-upload.test.ts`
- Modify: `frontend/packages/shared/src/index.ts`

- [ ] **Step 1: 写自定义上传地址测试**

新增测试调用：

```ts
await uploadImageAsset(file, {
  endpoint: '/api/merchant/store/assets',
  fields: { asset_type: 'logo' },
})
```

断言 `fetch` 收到指定 endpoint、`FormData` 同时含 `image` 和 `asset_type`，响应 `image_url` 原样返回；保留 `uploadProductImage` 现有测试以防回归。

- [ ] **Step 2: 确认测试失败**

Run: `cd frontend && npm run test -w packages/shared -- --run src/product-image-upload.test.ts`

Expected: `uploadImageAsset` 尚未导出。

- [ ] **Step 3: 抽取通用函数并添加共享类型**

实现：

```ts
export async function uploadImageAsset(
  file: File,
  options: { endpoint: string; fields?: Record<string, string> },
): Promise<string> {
  if (!file || file.size === 0) throw new Error('请选择图片文件')
  if (file.size > MAX_PRODUCT_IMAGE_BYTES) throw new Error('图片不能超过 5 MB')
  if (!file.type.startsWith('image/')) throw new Error('只支持图片文件')
  const token = getToken()
  if (!token) throw new Error('登录已失效，请重新登录')
  const body = new FormData()
  body.append('image', file)
  Object.entries(options.fields ?? {}).forEach(([key, value]) => body.append(key, value))
  const response = await fetch(options.endpoint, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body,
  })
  const payload = await response.json().catch(() => ({})) as UploadEnvelope
  if (!response.ok) throw new Error(payload.message || '图片上传失败')
  const imageURL = payload.data?.image_url || payload.image_url || ''
  if (!imageURL) throw new Error('图片上传响应缺少地址')
  return imageURL
}

export const uploadProductImage = (file: File, endpoint: string) =>
  uploadImageAsset(file, { endpoint })
```

在 `types.ts` 增加 `MerchantStoreProfile`、`StoreDetailResp`、`StoreProductsResp`、`ShowcaseSlot`、`ShowcaseResp`、`ShowcaseCandidate`、`ShowcaseCandidatesResp`、`ShowcasePublishReq`，并给现有 `ProductCard` 增加商家字段；从 `index.ts` 导出。

- [ ] **Step 4: 运行共享测试和类型消费构建**

Run: `cd frontend && npm run test -w packages/shared -- --run src/product-image-upload.test.ts`

Expected: PASS。

Run: `cd frontend && npm run build:shop`

Expected: TypeScript 编译成功。

- [ ] **Step 5: 提交共享契约**

```bash
git add frontend/packages/shared/src/types.ts \
  frontend/packages/shared/src/product-image-upload.ts \
  frontend/packages/shared/src/product-image-upload.test.ts \
  frontend/packages/shared/src/index.ts
git commit -m "feat: 共享店铺与橱窗前端契约"
```

### Task 9: 实现商品详情页和店铺页路由

**Files:**
- Create: `frontend/packages/shop/src/App.test.tsx`
- Create: `frontend/packages/shop/src/pages/ProductDetailPage.tsx`
- Create: `frontend/packages/shop/src/pages/ProductDetailPage.test.tsx`
- Create: `frontend/packages/shop/src/pages/StorePage.tsx`
- Create: `frontend/packages/shop/src/pages/StorePage.test.tsx`
- Modify: `frontend/packages/shop/src/App.tsx`
- Modify: `frontend/packages/shop/src/styles/shop.css`

- [ ] **Step 1: 写页面路由和数据加载测试**

在 jsdom 中用 `window.history.replaceState` 分别设置 `/shop`、`/product/100`、`/store/1000`，断言 App 渲染正确页面；详情页 mock `/api/shop/products/detail?product_id=100`，店铺页 mock 带 `merchant_id` 的 detail 与 products 两个请求。覆盖加载、404、空店铺和分页。

- [ ] **Step 2: 确认测试失败**

Run: `cd frontend && npm run test -w packages/shop -- --run src/App.test.tsx src/pages/ProductDetailPage.test.tsx src/pages/StorePage.test.tsx`

Expected: 页面组件不存在或 App 不识别新路径。

- [ ] **Step 3: 实现不依赖服务端 router 的路径状态机**

现有单文件打包继续使用 `window.location`，路径解析固定为：

```ts
type ShopRoute =
  | { kind: 'home' }
  | { kind: 'orders' }
  | { kind: 'payment' }
  | { kind: 'product'; productId: number }
  | { kind: 'store'; merchantId: number }

export function parseShopRoute(pathname: string): ShopRoute {
  const product = pathname.match(/^\/product\/(\d+)\/?$/)
  if (product) return { kind: 'product', productId: Number(product[1]) }
  const store = pathname.match(/^\/store\/(\d+)\/?$/)
  if (store) return { kind: 'store', merchantId: Number(store[1]) }
  if (pathname === '/pay') return { kind: 'payment' }
  return { kind: 'home' }
}
```

提供 `navigateShop(path)` 调用 `history.pushState` 并派发本地事件；App 同时监听该事件和 `popstate`，确保浏览器前进后退有效。

- [ ] **Step 4: 实现页面**

详情页展示商品图、价格、库存状态、商品 ID、店铺 Logo/名称、“进入店铺”、“立即购买”和最多 4 件同店启用商品。店铺页头部展示横幅、Logo、店铺名、简介；商品网格只消费后端已过滤的上架列表。空横幅和 Logo 使用 CSS 渐变横幅与店铺名首字作为默认视觉，禁止把空 URL 传给 `<img>`。

- [ ] **Step 5: 运行测试**

Run: `cd frontend && npm run test -w packages/shop -- --run src/App.test.tsx src/pages/ProductDetailPage.test.tsx src/pages/StorePage.test.tsx`

Expected: PASS。

- [ ] **Step 6: 提交公开页面**

```bash
git add frontend/packages/shop/src/App.tsx \
  frontend/packages/shop/src/App.test.tsx \
  frontend/packages/shop/src/pages/ProductDetailPage.tsx \
  frontend/packages/shop/src/pages/ProductDetailPage.test.tsx \
  frontend/packages/shop/src/pages/StorePage.tsx \
  frontend/packages/shop/src/pages/StorePage.test.tsx \
  frontend/packages/shop/src/styles/shop.css
git commit -m "feat: 增加商品详情与商家店铺页面"
```

### Task 10: 改造首页商品卡片的三种动作

**Files:**
- Modify: `frontend/packages/shop/src/components/ProductCard.tsx`
- Modify: `frontend/packages/shop/src/components/ProductCard.test.tsx`
- Modify: `frontend/packages/shop/src/pages/HomePage.tsx`
- Modify: `frontend/packages/shop/src/styles/shop.css`

- [ ] **Step 1: 先写交互隔离测试**

组件 props 改为：

```ts
type ProductCardProps = {
  product: ProductCardData
  onOpenProduct: (productId: number) => void
  onOpenStore: (merchantId: number) => void
  onBuy: (product: ProductCardData) => void
}
```

分别点击卡片主体、店铺链接和购买按钮，断言每次只触发对应回调一次；点击售罄商品购买按钮不触发 `onBuy`，但仍允许进入详情和店铺。

- [ ] **Step 2: 确认测试失败**

Run: `cd frontend && npm run test -w packages/shop -- --run src/components/ProductCard.test.tsx`

Expected: 当前卡片没有独立详情与店铺回调。

- [ ] **Step 3: 实现三个互不串扰的操作区**

卡片主体使用语义化 `<button>` 或键盘可访问容器进入 `/product/{id}`；店铺链接必须 `event.stopPropagation()` 后进入 `/store/{merchant_id}`；购买按钮继续打开现有支付弹窗。HomePage 使用 Task 9 的 `navigateShop`，不整页刷新。

商品卡片上展示 `merchant_name`；没有商家名时显示“平台自营”，但后端正常数据必须返回真实商家。

- [ ] **Step 4: 运行测试和首页测试**

Run: `cd frontend && npm run test -w packages/shop -- --run src/components/ProductCard.test.tsx src/pages/ProductDetailPage.test.tsx src/pages/StorePage.test.tsx`

Expected: PASS。

- [ ] **Step 5: 提交首页导航**

```bash
git add frontend/packages/shop/src/components/ProductCard.tsx \
  frontend/packages/shop/src/components/ProductCard.test.tsx \
  frontend/packages/shop/src/pages/HomePage.tsx \
  frontend/packages/shop/src/styles/shop.css
git commit -m "feat: 打通首页商品与店铺导航"
```

### Task 11: 增加商家端店铺设置页面

**Files:**
- Create: `frontend/packages/merchant/src/pages/StoreSettingsPage.tsx`
- Create: `frontend/packages/merchant/src/pages/StoreSettingsPage.test.tsx`
- Modify: `frontend/packages/merchant/src/App.tsx`
- Modify: `frontend/packages/merchant/src/App.test.tsx`

- [ ] **Step 1: 写页面状态测试**

mock GET profile、两次图片上传和 POST profile，覆盖：初次资料 `version=0`、上传 Logo、上传横幅、简介计数、保存成功刷新 version、HTTP 409 显示“资料已被更新，请刷新后重试”。

App 测试断言侧栏出现“店铺设置”，路径 `/merchant/store` 渲染该页。

- [ ] **Step 2: 确认测试失败**

Run: `cd frontend && npm run test -w packages/merchant -- --run src/pages/StoreSettingsPage.test.tsx src/App.test.tsx`

Expected: 页面和菜单不存在。

- [ ] **Step 3: 实现资料表单和上传**

页面使用 Ant Design `Form`、`Upload` 和统一模板实时预览；预览与公开店铺页复用相同的横幅比例、Logo 尺寸、默认视觉和简介排版。上传分别调用：

```ts
uploadImageAsset(file, {
  endpoint: '/api/merchant/store/assets',
  fields: { asset_type: 'logo' },
})
```

以及 `asset_type: 'banner'`。保存请求必须携带当前 `version` 为 `expected_version`；保存期间禁用按钮，失败不清空本地编辑内容。

- [ ] **Step 4: 运行测试和构建**

Run: `cd frontend && npm run test -w packages/merchant -- --run src/pages/StoreSettingsPage.test.tsx src/App.test.tsx`

Expected: PASS。

Run: `cd frontend && npm run build:merchant`

Expected: 构建成功。

- [ ] **Step 5: 提交商家店铺设置**

```bash
git add frontend/packages/merchant/src/pages/StoreSettingsPage.tsx \
  frontend/packages/merchant/src/pages/StoreSettingsPage.test.tsx \
  frontend/packages/merchant/src/App.tsx \
  frontend/packages/merchant/src/App.test.tsx
git commit -m "feat: 增加商家店铺设置入口"
```

### Task 12: 增加管理员首页橱窗工作台

**Files:**
- Create: `frontend/packages/admin/src/pages/ShowcasePage.tsx`
- Create: `frontend/packages/admin/src/pages/ShowcasePage.test.tsx`
- Modify: `frontend/packages/admin/src/App.tsx`
- Modify: `frontend/packages/admin/src/App.test.tsx`

- [ ] **Step 1: 写布局编辑测试**

mock layout 和 candidates，覆盖：始终渲染 12 槽；候选加入第一个空槽；重复商品不加入；同商家第三件在 UI 禁用；移除后槽位保留为空；上下移动和拖放交换槽位；失效槽显示原因；发布携带当前 version 且只提交非空槽位；409 后提示刷新而不覆盖草稿。

- [ ] **Step 2: 确认测试失败**

Run: `cd frontend && npm run test -w packages/admin -- --run src/pages/ShowcasePage.test.tsx src/App.test.tsx`

Expected: 页面和菜单不存在。

- [ ] **Step 3: 实现布局模型和可访问排序**

页面状态始终是长度 12 的数组：

```ts
type DraftSlot = ShowcaseSlot & { dirty?: boolean }

function moveSlot(items: DraftSlot[], from: number, to: number): DraftSlot[] {
  const next = [...items]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next.map((slot, index) => ({ ...slot, slot_no: index + 1, dirty: true }))
}
```

支持原生 HTML5 drag/drop，同时提供“上移/下移”按钮供键盘和测试使用，不新增拖拽依赖。候选区展示总分及五项拆分，明确标记“仅建议，不会自动发布”。

- [ ] **Step 4: 实现发布**

发布体由 12 槽草稿过滤出非空项；缺失位置即为发布后的空槽：

```ts
const payload: ShowcasePublishReq = {
  expected_version: version,
  items: slots
    .filter(({ product_id }) => product_id > 0)
    .map(({ slot_no, product_id }) => ({ slot_no, product_id })),
}
```

成功后使用服务端响应替换草稿和 version；失败保持草稿。菜单路径固定 `/admin/showcase`。

- [ ] **Step 5: 运行测试和构建**

Run: `cd frontend && npm run test -w packages/admin -- --run src/pages/ShowcasePage.test.tsx src/App.test.tsx`

Expected: PASS。

Run: `cd frontend && npm run build:admin`

Expected: 构建成功。

- [ ] **Step 6: 提交管理员工作台**

```bash
git add frontend/packages/admin/src/pages/ShowcasePage.tsx \
  frontend/packages/admin/src/pages/ShowcasePage.test.tsx \
  frontend/packages/admin/src/App.tsx \
  frontend/packages/admin/src/App.test.tsx
git commit -m "feat: 增加管理员首页橱窗工作台"
```

### Task 13: 增加观测指标并更新迁移状态

**Files:**
- Create: `app/gateway/hertz/internal/handler/showcase_metrics.go`
- Create: `app/gateway/hertz/internal/handler/metrics_handler.go`
- Modify: `app/gateway/hertz/internal/handler/showcase.go`
- Modify: `app/gateway/hertz/internal/handler/showcase_recommendation.go`
- Modify: `app/gateway/hertz/internal/handler/migration_status.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`

- [ ] **Step 1: 写迁移状态和指标更新测试**

断言迁移状态包含本计划涉及的 10 个 API、`/metrics` 和两个页面路由；用 Prometheus testutil 验证公开读取成功、发布成功/冲突、当前有效商品数、待补位槽数、各失效原因、候选成功/失败与耗时会更新对应 collector。

- [ ] **Step 2: 确认测试失败**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test(ShowcaseMetrics|RouteMigrationStatus)' -count=1`

Expected: collectors 或迁移条目不存在。

- [ ] **Step 3: 实现低基数指标**

指标固定为：

```text
flashmall_showcase_read_total{audience="public|admin",result="success|error"}
flashmall_showcase_publish_total{result="success|conflict|invalid|error"}
flashmall_showcase_active_items
flashmall_showcase_pending_slots
flashmall_showcase_invalid_slots{reason="product_not_found|product_inactive|merchant_not_found|merchant_inactive|out_of_stock"}
flashmall_showcase_candidate_duration_seconds
flashmall_showcase_candidate_total{result="success|error"}
flashmall_store_request_duration_seconds{operation="detail|products",result="success|error"}
flashmall_showcase_request_duration_seconds{audience="public|admin",result="success|error"}
```

直方图用于计算店铺详情、店铺商品、首页和后台橱窗接口 p95；gauge 表示当前有效商品数与待补位槽数。不要把 `product_id`、`merchant_id`、`operator_id` 放入 label，防止高基数。发布日志使用现有 trace/request ID，结构化记录 operator ID、expected/current/new version 和耗时；店铺资料修改日志记录 merchant ID、user ID、结果和 request ID，且不记录 token 或上传内容。

`MetricsHandler` 使用 `prometheus.DefaultGatherer.Gather()` 与 `expfmt.MetricFamilyToText` 输出 `text/plain; version=0.0.4`，并在 system routes 注册 `GET /metrics`。它只暴露聚合指标，不输出令牌、请求体、商品 ID 或商家 ID。

- [ ] **Step 4: 更新迁移状态**

将新 API 标为 `hertz-native`，说明商品读仍为 `go-zero product-rpc`、库存写仍为 `kitex inventory command`。这一描述用于面试演示技术边界，不把旧 entry-api 标记为已删除。

- [ ] **Step 5: 运行测试**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test(ShowcaseMetrics|RouteMigrationStatus|Routes_Showcase|Routes_Store)' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交观测与迁移说明**

```bash
git add app/gateway/hertz/internal/handler/showcase_metrics.go \
  app/gateway/hertz/internal/handler/metrics_handler.go \
  app/gateway/hertz/internal/handler/showcase.go \
  app/gateway/hertz/internal/handler/showcase_recommendation.go \
  app/gateway/hertz/internal/handler/migration_status.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go
git commit -m "feat: 补齐橱窗观测与迁移状态"
```

### Task 14: 生成三端静态资源并完成代码级总验证

**Files:**
- Modify: `app/entry/api/internal/handler/web/shop.html`
- Modify: `app/entry/api/internal/handler/web/merchant.html`
- Modify: `app/entry/api/internal/handler/web/admin.html`
- Modify: `docs/superpowers/specs/2026-07-13-storefront-showcase-design.md`（仅当实现中发现已确认规格与真实依赖冲突时，记录等价修正；不得扩张需求）

- [ ] **Step 1: 格式化和生成前端产物**

Run: `gofmt -w app/gateway/hertz/internal/handler/*.go`

Run: `cd frontend && npm run build`

Expected: shop、merchant、admin 三个单文件 HTML 更新到 Hertz 当前使用的静态目录。

- [ ] **Step 2: 运行后端完整单元测试**

Run: `go test ./app/gateway/hertz/internal/handler ./app/gateway/hertz/internal/middleware ./app/gateway/hertz/internal/svc -count=1`

Expected: PASS，无竞态相关共享全局测试污染。

- [ ] **Step 3: 运行前端完整测试**

Run: `cd frontend && npm test -- --run`

Expected: shared、shop、merchant、admin 全部 PASS。

- [ ] **Step 4: 验证生成产物同步**

Run: `node scripts/ci/check-web-artifacts.mjs`

Expected: PASS，不存在源码已改但提交旧 HTML 的情况。

- [ ] **Step 5: 检查差异和敏感信息**

Run: `git diff --check`

Expected: 无空白错误。

Run: `git grep -nE '(Bearer [A-Za-z0-9._-]+|password["'"']?\s*[:=]\s*["'"'][^"'"']+)' -- ':!docs/superpowers/plans/*' || true`

Expected: 不出现新提交 token 或真实密码；已有本地演示配置需逐项确认不是本任务新增。

- [ ] **Step 6: 提交静态产物**

```bash
git add app/entry/api/internal/handler/web/shop.html \
  app/entry/api/internal/handler/web/merchant.html \
  app/entry/api/internal/handler/web/admin.html
git commit -m "build: 更新店铺与橱窗静态页面"
```

### Task 15: 在 WSL Docker 中迁移、重建并走完整链路

**Files:**
- Modify: `scripts/ci/smoke-e2e.sh`（增加稳定且可重复的店铺/橱窗只读检查）
- Modify: `docs/CURRENT_PROJECT.md`（记录本轮真实结果，不读取桌面旧日志作为实现依据）

- [ ] **Step 1: 给 smoke 脚本增加先失败的接口检查**

在不修改业务数据的 smoke 段加入：

```bash
curl -fsS "${BASE_URL}/api/shop/catalog" | grep -q '"items"'
curl -fsS "${BASE_URL}/api/shop/stores/detail?merchant_id=1000" | grep -q '"merchant_id":1000'
curl -fsS "${BASE_URL}/product/100" | grep -qi '<!doctype html>'
curl -fsS "${BASE_URL}/store/1000" | grep -qi '<!doctype html>'
```

先对旧容器运行并确认至少新店铺 API 失败，证明 smoke 确实覆盖本轮功能。

- [ ] **Step 2: 应用数据库迁移**

Run: `docker compose -f deploy/docker-compose.yml run --rm mysql-init`

Expected: 迁移成功；重复运行一次仍成功；现有商品 `create_time` 不晚于当前时间 31 天左右；若橱窗已有人工 item，则种子没有覆盖。

用 Compose 内的 MySQL 客户端执行以下 SQL 验证：

```sql
SELECT COUNT(*) FROM mall_order.merchant_store_profile;
SELECT version FROM mall_product.homepage_showcase WHERE id=1;
SELECT slot_no, product_id FROM mall_product.homepage_showcase_item WHERE showcase_id=1 ORDER BY slot_no;
SELECT id, create_time FROM mall_product.product ORDER BY id LIMIT 10;
```

Run: `docker compose -f deploy/docker-compose.yml exec -T mysql sh -lc 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "SELECT COUNT(*) FROM mall_order.merchant_store_profile; SELECT version FROM mall_product.homepage_showcase WHERE id=1; SELECT slot_no, product_id FROM mall_product.homepage_showcase_item WHERE showcase_id=1 ORDER BY slot_no; SELECT id, create_time FROM mall_product.product ORDER BY id LIMIT 10;"'`

Expected: 表存在、全局橱窗存在、已有人工布局未被覆盖、旧商品时间不被回填成刚创建。

- [ ] **Step 3: 只重建受影响镜像**

Run: `./scripts/local/build-compose-images.sh --tag dev hertz-gateway`

Expected: 只重建 Hertz gateway，不重复构建无关 RPC 服务。

Run: `docker compose -f deploy/docker-compose.yml up -d --no-deps hertz-gateway`

Expected: 容器进入 healthy，端口 8889 可访问。

- [ ] **Step 4: 执行自动 smoke**

Run: `BASE_URL=http://127.0.0.1:8889 ./scripts/ci/smoke-e2e.sh`

Expected: PASS，公开首页、店铺、详情和现有下单链路均未回归。

- [ ] **Step 5: 用真实角色走写链路**

按本地演示账号经 `/api/auth/login` 获取商家和管理员 token，不把 token 写入仓库。依次执行：

```text
商家登录
GET  /api/merchant/store/profile
POST /api/merchant/store/assets 上传 Logo
POST /api/merchant/store/assets 上传横幅
POST /api/merchant/store/profile 携带 expected_version 保存
POST /api/merchant/products/create 创建一件新商品
POST /api/merchant/products/update 将新商品状态设为 1
GET  /api/shop/stores/detail?merchant_id={merchant_id}
GET  /api/shop/stores/products?merchant_id={merchant_id} 验证新商品已在店内
GET  /api/shop/catalog 验证新商品尚未自动进入首页

管理员登录
GET  /api/admin/showcase 保存原 version 与布局用于验收后恢复
GET  /api/admin/showcase/candidates 验证能找到新商品及中文推荐原因
POST /api/admin/showcase/publish 将新商品加入空槽，携带非空槽位和 expected_version，完整替换旧布局
GET  /api/shop/catalog 验证顺序
再次用旧 expected_version 发布，确认 HTTP 409
```

再临时下架这件已占槽商品，验证公开首页隐藏而 `/api/admin/showcase` 保留原槽并返回 `product_inactive`；恢复商品状态后，用最新 version 发布步骤开始时保存的原布局，避免污染现有演示橱窗。

- [ ] **Step 6: 用 Chrome 完成三端可视验收**

在同一 Chrome 会话依次打开：

```text
http://127.0.0.1:8889/shop
http://127.0.0.1:8889/product/100
http://127.0.0.1:8889/store/1000
http://127.0.0.1:8889/merchant/store
http://127.0.0.1:8889/admin/showcase
```

验收：图片无空白；卡片主体、店铺链接、购买按钮互不串扰；店铺资料保存后公开端即时可见；管理员候选有分数解释；发布后首页顺序更新；浏览器前进后退正确；控制台无未处理异常和 404 资源。

- [ ] **Step 7: 检查指标与日志**

Run: `curl -fsS http://127.0.0.1:8889/metrics | grep 'flashmall_showcase_'`

Expected: 能看到 read、publish、invalid_slots 和 candidate_duration 指标。

Run: `docker compose -f deploy/docker-compose.yml logs --since=10m hertz-gateway`

Expected: 有发布结构化日志和 request ID，无 panic、SQL 重试风暴或 Kitex 首页调用。

- [ ] **Step 8: 更新唯一有效项目状态文档**

只记录真实执行结果：新增入口、通过的命令、容器镜像、可视验收、已知限制。不要从桌面日志复制实现状态，也不要新建第二份重复状态文档。

- [ ] **Step 9: 最终提交**

```bash
git add scripts/ci/smoke-e2e.sh docs/CURRENT_PROJECT.md
git commit -m "docs: 记录店铺与橱窗验收结果"
```

- [ ] **Step 10: 最终分支检查**

Run: `git status --short --branch`

Expected: 工作区干净，分支只领先 `origin/codex/desktop-launcher`，没有合并到 `main`。

Run: `git log --oneline --decorate -15`

Expected: 本计划的数据库、后端、三端页面、观测、产物和验收提交按任务顺序存在。

---

## 最终验收矩阵

| 场景 | 预期结果 |
|---|---|
| 商家首次打开店铺设置 | 返回商家名、空素材、`version=0`，可直接创建资料 |
| 商家并发保存资料 | 第一个成功，旧版本请求 409，不静默覆盖 |
| 商家上架商品 | 出现在自己的店铺，不自动进入首页 |
| 管理员发布 12 槽 | 完整事务提交、版本 +1、产生审计和指标 |
| 同商家选择第 3 件 | 前端禁用，后端再次拒绝 |
| 首页槽商品下架 | 首页隐藏，管理员仍见原槽及原因，不自动补位 |
| 推荐候选 | 分数拆分可解释，同一输入排序稳定，不自动发布 |
| 首页/店铺库存展示 | 使用快照/缓存，不增加 Kitex 调用 |
| 下单结算 | 继续以 Kitex 预占结果为准 |
| 旧 entry-api | 保留对照配置和代码，不被新功能反向污染 |
| 三端静态页面 | Hertz 8889 直接可访问，无空白页和资源 404 |

## 完成定义

只有同时满足以下条件才可宣布完成：全部 15 个任务的聚焦测试通过；后端与前端完整测试通过；三份静态 HTML 已重新生成；WSL Docker 中实际迁移和重建成功；公开、商家、管理员写读链路各走通一次；Chrome 验证无空白页和控制台错误；失效槽位、版本冲突、商家配额三个异常场景已实测；项目状态文档记录的是本轮真实证据；工作区干净且没有创建或合并到 `main` 的 PR。

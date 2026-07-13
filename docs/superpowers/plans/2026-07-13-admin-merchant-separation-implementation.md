# 平台后台与商家后台独立入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 恢复商品自定义图片闭环，并把平台管理员后台与商家后台拆成 `/admin` 和 `/merchant` 两个具有独立权限边界的 React 入口。

**Architecture:** 保留 Hertz 作为统一 HTTP 网关，在服务端分别注册管理员与商家静态入口和 API 中间件。前端新增 merchant workspace，复用 shared 中的认证、请求、类型及图片上传能力；管理员与商家页面在编译期绑定各自 API 前缀，不再通过 workspace 状态切换权限。

**Tech Stack:** Go 1.24、CloudWeGo Hertz、React 19、TypeScript、Ant Design Pro、Vite singlefile、Vitest、.NET 8 WPF、Chrome/Playwright。

---

## 文件结构

- `frontend/packages/shared/src/product-image-upload.ts`：图片前端校验和 multipart 上传。
- `frontend/packages/shared/src/types.ts`：补齐 `image_url`、商家概览、库存、退款类型。
- `frontend/packages/admin/src/pages/ProductsPage.tsx`：恢复管理员图片预览、URL 和上传。
- `frontend/packages/merchant/`：独立商家应用、权限守卫和五个领域页面。
- `app/gateway/hertz/internal/handler/routes.go`：注册 `/merchant` 和商家图片上传。
- `app/gateway/hertz/internal/handler/admin_product_image.go`：复用的受保护图片上传实现。
- `frontend/build.js`、`scripts/ci/check-web-artifacts.mjs`、`.github/workflows/ci.yml`：第三个前端产物的构建与语法门禁。
- `tools/FlashMall.Launcher/`：控制中心新增商家后台入口。

### Task 1: 共享图片上传能力与管理员图片回归

**Files:**
- Create: `frontend/packages/shared/src/product-image-upload.ts`
- Create: `frontend/packages/shared/src/product-image-upload.test.ts`
- Modify: `frontend/packages/shared/src/index.ts`
- Modify: `frontend/packages/shared/src/types.ts`
- Create: `frontend/packages/admin/src/pages/ProductsPage.test.tsx`
- Modify: `frontend/packages/admin/src/pages/ProductsPage.tsx`

- [ ] **Step 1: 写共享上传函数失败测试**

在测试中使用真实 `File` 和拦截后的 `fetch`，断言字段名为 `image`、请求发送到传入端点，并返回网关 envelope 中的 `image_url`：

```ts
const file = new File(['png'], 'coat.png', { type: 'image/png' });
const imageURL = await uploadProductImage(file, '/api/admin/products/image');
expect(imageURL).toBe('/uploads/products/coat.png');
expect(requestURL).toBe('/api/admin/products/image');
expect(requestBody.get('image')).toBe(file);
```

- [ ] **Step 2: 运行 RED**

Run: `cd frontend && npm test -w packages/shared -- --run src/product-image-upload.test.ts`

Expected: FAIL，原因是 `uploadProductImage` 尚不存在。

- [ ] **Step 3: 实现最小上传函数**

实现 `uploadProductImage(file, endpoint)`：拒绝空文件、超过 5 MB 和非 `image/*` 类型；构造 `FormData`；从 `getToken()` 读取 token；直接调用 `fetch` 以避免 JSON API 客户端覆盖 multipart content-type；兼容 Hertz `{code,data}` envelope；非 2xx 抛出带服务端 message 的 `Error`。

- [ ] **Step 4: 运行 GREEN 并导出函数**

Run: `cd frontend && npm test -w packages/shared -- --run src/product-image-upload.test.ts`

Expected: PASS。随后从 `frontend/packages/shared/src/index.ts` 导出函数及 `MAX_PRODUCT_IMAGE_BYTES`。

- [ ] **Step 5: 写管理员商品表单失败测试**

渲染 `ProductsPage`，拦截供应商与商品列表请求，点击“新增商品”，断言存在“图片地址”“上传图片”和预览区域；选择文件并保存后，断言创建请求包含：

```ts
expect(createBody).toMatchObject({ image_url: '/uploads/products/admin.png' });
```

- [ ] **Step 6: 运行管理员 RED**

Run: `cd frontend && npm test -w packages/admin -- --run src/pages/ProductsPage.test.tsx`

Expected: FAIL，当前表单和类型均无 `image_url`。

- [ ] **Step 7: 恢复管理员图片字段**

给 `AdminProductItem`、`AdminProductCreateReq`、`AdminProductUpdateReq` 增加 `image_url`。在商品表格增加缩略图列；在新增/编辑 Modal 增加 URL 输入、文件选择、上传中状态和预览；保存时先上传文件，再提交最终 `image_url`。上传失败时保留 Modal 和其他字段。

- [ ] **Step 8: 运行管理员 GREEN**

Run: `cd frontend && npm test -w packages/admin -- --run src/pages/ProductsPage.test.tsx`

Expected: PASS。

- [ ] **Step 9: 提交共享与管理员图片恢复**

```bash
git add frontend/packages/shared frontend/packages/admin/src/pages/ProductsPage.tsx frontend/packages/admin/src/pages/ProductsPage.test.tsx
git commit -m "fix: 恢复后台商品自定义图片"
```

### Task 2: Hertz 商家入口与商家图片上传权限

**Files:**
- Create: `app/gateway/hertz/internal/handler/product_image_test.go`
- Modify: `app/gateway/hertz/internal/handler/admin_product_image.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`
- Modify: `scripts/k8s/init-db.sql`

- [ ] **Step 1: 写路由与上传权限失败测试**

扩展路由测试，要求存在以下路由且中间件边界不同：

```text
GET  /merchant
GET  /merchant/*any
POST /api/merchant/products/image
```

上传测试分别验证：无 token 返回 401；普通已登录但无商家绑定的用户不能通过后续商家校验；合法商家上传 PNG 返回 `/uploads/products/` 地址；伪图片返回 400。

- [ ] **Step 2: 运行 RED**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test.*(MerchantPage|MerchantProductImage)' -count=1`

Expected: FAIL，商家页面与上传路由尚未注册。

- [ ] **Step 3: 提取可复用上传处理器**

将文件处理主体改成私有 `productImageUploadHandler(svcCtx)`；`AdminProductImageUploadHandler` 和 `MerchantProductImageUploadHandler` 均返回该处理器。保留服务端 5 MB、MIME、扩展名、随机文件名与持久化目录约束。

- [ ] **Step 4: 注册 Hertz 路由**

在系统路由中注册 `/merchant` 与 `/merchant/*any` 返回 `merchant.html`；在商家路由中用 `RequireMerchant` 注册 `POST /api/merchant/products/image`。上传请求进入处理器前先验证该用户存在启用的 `merchant_user` 绑定，不能仅依赖“有 JWT”。

本地演示数据幂等绑定用户 `1001` 到自营商家 `1000`，用于实际验收：手机号 `13800000001`，密码沿用 `DemoPassword`（默认 `flashmall123`）。不得赋予该用户 admin role。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test.*(MerchantPage|MerchantProductImage)' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交 Hertz 边界**

```bash
git add app/gateway/hertz/internal/handler
git commit -m "feat: 增加独立商家后台入口"
```

### Task 3: 商家应用骨架、登录与权限守卫

**Files:**
- Create: `frontend/packages/merchant/package.json`
- Create: `frontend/packages/merchant/tsconfig.json`
- Create: `frontend/packages/merchant/vite.config.ts`
- Create: `frontend/packages/merchant/index.html`
- Create: `frontend/packages/merchant/src/main.tsx`
- Create: `frontend/packages/merchant/src/App.tsx`
- Create: `frontend/packages/merchant/src/components/MerchantGuard.tsx`
- Create: `frontend/packages/merchant/src/components/MerchantGuard.test.tsx`
- Create: `frontend/packages/merchant/src/pages/LoginPage.tsx`
- Create: `frontend/packages/merchant/src/pages/DashboardPage.tsx`
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`

- [ ] **Step 1: 写 MerchantGuard 失败测试**

覆盖三种真实行为：无 token 显示商家登录；有效 token 但 `/api/merchant/me` 返回 403 显示“未绑定可用商家”；返回商家列表时渲染 children。测试不得只断言 mock 调用次数，应断言用户可见结果。

- [ ] **Step 2: 运行 RED**

Run: `cd frontend && npm test -w packages/merchant -- --run src/components/MerchantGuard.test.tsx`

Expected: FAIL，workspace 和守卫尚不存在。

- [ ] **Step 3: 创建独立 workspace**

使用 admin 相同版本的 React、Ant Design、Vite singlefile 和 Vitest 配置；开发端口使用 `3002`；代理 `/api` 到 `http://localhost:8889`。根 `package.json` 增加 workspace 以及 `dev:merchant`、`build:merchant`。

- [ ] **Step 4: 实现登录与权限守卫**

登录页复用 `/api/auth/login` 和 shared token 存储，不显示管理员一键登录。守卫先检查 token 有效性，再调用 `/api/merchant/me`；401 清理身份，403 或空 items 显示无商家状态，成功后进入应用。

- [ ] **Step 5: 实现独立布局与概览**

`App.tsx` 菜单固定为 `/merchant`、`/merchant/products`、`/merchant/inventory`、`/merchant/orders`、`/merchant/refunds`。概览读取 `/api/merchant/dashboard/stats`，只呈现服务端已有字段，并通过 `flash-merchant:navigate` 导航，不复用 `flash-admin:navigate`。

- [ ] **Step 6: 运行 GREEN 与构建**

Run: `cd frontend && npm test -w packages/merchant -- --run src/components/MerchantGuard.test.tsx && npm run build:merchant`

Expected: PASS，且生成 `frontend/packages/merchant/dist/index.html`。

- [ ] **Step 7: 提交应用骨架**

```bash
git add frontend/package.json frontend/package-lock.json frontend/packages/merchant
git commit -m "feat: 建立独立商家后台应用"
```

### Task 4: 商家商品、库存、订单和退款页面

**Files:**
- Modify: `frontend/packages/shared/src/types.ts`
- Create: `frontend/packages/merchant/src/pages/ProductsPage.tsx`
- Create: `frontend/packages/merchant/src/pages/ProductsPage.test.tsx`
- Create: `frontend/packages/merchant/src/pages/InventoryPage.tsx`
- Create: `frontend/packages/merchant/src/pages/OrdersPage.tsx`
- Create: `frontend/packages/merchant/src/pages/OrdersPage.test.tsx`
- Create: `frontend/packages/merchant/src/pages/RefundsPage.tsx`
- Modify: `frontend/packages/merchant/src/App.tsx`

- [ ] **Step 1: 写商品租户边界失败测试**

渲染商家商品页，完成列表、上传、创建和库存调整操作；记录全部请求 URL 并断言：

```ts
expect(urls).toContain('/api/merchant/products');
expect(urls).toContain('/api/merchant/products/image');
expect(urls).toContain('/api/merchant/products/create');
expect(urls).toContain('/api/merchant/products/stock-adjust');
expect(urls.some((url) => url.startsWith('/api/admin/'))).toBe(false);
```

- [ ] **Step 2: 运行商品 RED**

Run: `cd frontend && npm test -w packages/merchant -- --run src/pages/ProductsPage.test.tsx`

Expected: FAIL，页面尚不存在。

- [ ] **Step 3: 实现商家商品和库存页面**

商品页包含缩略图、图片 URL、文件上传、创建、编辑、上/下架和 Kitex 库存调整。供应商只能从商品已有 supplier 信息或平台只读列表选择；所有写请求必须使用 merchant 前缀。库存页读取 `/api/merchant/inventory/stock-changes`，显示 before/after、delta、reason、request_id 与时间。

- [ ] **Step 4: 运行商品 GREEN**

Run: `cd frontend && npm test -w packages/merchant -- --run src/pages/ProductsPage.test.tsx`

Expected: PASS。

- [ ] **Step 5: 写订单权限失败测试**

断言订单列表调用 `/api/merchant/orders`；已支付订单显示“发货”并调用 `/api/merchant/orders/ship`；不显示管理员关闭、退款执行、安全日志和用户管理操作。

- [ ] **Step 6: 运行订单 RED**

Run: `cd frontend && npm test -w packages/merchant -- --run src/pages/OrdersPage.test.tsx`

Expected: FAIL。

- [ ] **Step 7: 实现订单与退款页面**

订单页支持筛选、金额与状态展示及已支付订单发货。退款页只读调用 `/api/merchant/refunds`，展示退款号、订单号、金额、状态、原因与时间；不提供平台审核按钮。

- [ ] **Step 8: 运行 merchant 全部测试**

Run: `cd frontend && npm test -w packages/merchant -- --run`

Expected: PASS，且捕获不到 `/api/admin/*` 请求。

- [ ] **Step 9: 提交商家业务页面**

```bash
git add frontend/packages/shared/src/types.ts frontend/packages/merchant/src
git commit -m "feat: 完成商家后台核心经营页面"
```

### Task 5: 三前端构建、CI 产物门禁和桌面入口

**Files:**
- Modify: `frontend/build.js`
- Modify: `scripts/ci/check-web-artifacts.mjs`
- Modify: `.github/workflows/ci.yml`
- Create: `app/entry/api/internal/handler/web/merchant.html`（构建生成）
- Modify: `tools/FlashMall.Launcher/ViewModels/MainWindowViewModel.cs`
- Modify: `tools/FlashMall.Launcher/MainWindow.xaml`
- Modify: `tools/FlashMall.Launcher.Tests/ViewModels/MainWindowViewModelTests.cs`

- [ ] **Step 1: 写构建产物失败检查**

先把默认产物目标加入 `merchant.html`，运行语法脚本确认因文件缺失而失败；桌面测试增加 `OpeningMerchantWritesAnOperationLog`，断言打开 `http://127.0.0.1:8889/merchant` 并追加操作日志。

- [ ] **Step 2: 运行 RED**

```bash
node scripts/ci/check-web-artifacts.mjs
dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -c Release --filter OpeningMerchantWritesAnOperationLog
```

Expected: 两项均 FAIL。

- [ ] **Step 3: 接入第三个前端产物**

`frontend/build.js` 在 shop/admin 之后构建 merchant 并复制为 `merchant.html`。CI 的路径过滤、npm build 命令和 artifact 语法检查覆盖 merchant workspace。语法脚本按每个文件独立维护失败状态，避免前一个失败导致后续成功文件不打印结果。

- [ ] **Step 4: 增加桌面控制中心入口**

为 ViewModel 增加 `OpenMerchantCommand`，调用现有 `OpenUrl("http://127.0.0.1:8889/merchant")`；在入口按钮区增加“商家后台”，保持现有商城、平台后台和支付入口不变。

- [ ] **Step 5: 构建并运行 GREEN**

```bash
cd frontend && npm run build
cd .. && node scripts/ci/check-web-artifacts.mjs
dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -c Release
```

Expected: 生成 shop/admin/merchant 三个可解析单文件；桌面测试全部通过。

- [ ] **Step 6: 提交交付集成**

```bash
git add frontend/build.js scripts/ci/check-web-artifacts.mjs .github/workflows/ci.yml app/entry/api/internal/handler/web/merchant.html tools/FlashMall.Launcher tools/FlashMall.Launcher.Tests
git commit -m "feat: 接入商家后台构建与桌面入口"
```

### Task 6: 完整构建、运行与 Chrome 业务验收

**Files:**
- Modify only if verification exposes a proven defect.

- [ ] **Step 1: 运行静态和单元验证**

```bash
cd frontend && npm test -- --run && npm run build
cd ..
node scripts/ci/check-web-artifacts.mjs
go test ./app/gateway/hertz/internal/handler ./app/gateway/hertz/internal/middleware -count=1
go test ./... -count=1
dotnet test tools/FlashMall.Launcher.Tests/FlashMall.Launcher.Tests.csproj -c Release
git diff --check
```

Expected: 全部退出码为 0，无 JavaScript 语法错误。

- [ ] **Step 2: 检查运行环境并重建 Hertz**

先运行控制协议状态命令，确认 8889 当前监听者和 Docker/WSL 状态；只重建 `hertz-gateway`，不无条件重启数据库、RPC 和消息队列：

```bash
./scripts/local/flash-mall-control.sh status
./scripts/local/flash-mall-control.sh rebuild-service hertz-gateway --wait-timeout 300
./scripts/local/flash-mall-control.sh status
```

Expected: 最终事件为 ready；三个页面 HTTP 200。

- [ ] **Step 3: 使用已安装 Chrome 验证平台管理员链路**

打开 `/admin`，执行演示管理员一键登录；进入商品管理，上传一张小型 PNG，创建或编辑测试商品；确认缩略图立即显示且 `/shop` 使用相同 `/uploads/products/...` 地址。验证 `/admin` 不出现商家菜单。

- [ ] **Step 4: 使用已安装 Chrome 验证商家链路**

打开 `/merchant`，用已绑定测试商家的账号登录；验证概览、商品、库存、订单和退款页面；上传自定义图片、保存商品并在商城查看；对已支付订单执行发货。浏览器网络记录不得出现 `/api/admin/*`。

- [ ] **Step 5: 验证越权拒绝**

用普通用户访问 `/merchant` 应显示无可用商家；用商家 token 请求管理员 API 应返回 403；尝试修改其他商家的商品应返回 404/403。管理员访问 `/admin` 正常，但 `/merchant` 不因管理员身份自动获得商家数据。

- [ ] **Step 6: 检查并提交最终修正**

若验收暴露缺陷，先写最小失败测试再修复并重跑相关命令。只暂存该失败测试和对应修复文件，再按实际范围提交；若未暴露缺陷则跳过此提交：

```bash
git status --short
git commit -m "fix: 收口后台拆分实际运行链路"
```

- [ ] **Step 7: 推送并观察现有 PR CI**

推送 `codex/desktop-launcher`，确保现有 PR 目标仍是 `codex/arch-hertz-kitex` 而不是 `main`；等待快速 CI 的 changes、frontend-build、desktop-launcher、go-checks 与 ci-config-check 全部成功。不得创建面向 `main` 的 PR。

## 完成标准

- `/admin` 与 `/merchant` 返回不同的 React 单文件应用。
- 两种身份在前端菜单、请求前缀和 Hertz 中间件三个层面隔离。
- 管理员与商家均能上传商品图片，商品保存后商城正确展示。
- 商家核心经营页面可用且不能访问其他商家数据。
- 三个前端产物、Go、桌面启动器和 CI 全部通过。
- 当前 PR 仍合入 `codex/arch-hertz-kitex`。

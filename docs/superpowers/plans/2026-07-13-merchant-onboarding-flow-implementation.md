# 商家注册与入驻闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/merchant` 内完成注册、入驻申请、状态查看、管理员审核和审核通过后进入经营后台的图形化闭环。

**Architecture:** 保留普通用户作为唯一认证身份，审核通过后用 `merchant_user` 赋予商家身份。Hertz 增加本人申请查询并加强申请、审核的一致性；React 商家端由 `MerchantGuard` 状态机分流到认证、入驻状态或经营后台，管理员端新增独立审核页。

**Tech Stack:** Go 1.24、Go-zero Auth API、CloudWeGo Hertz、MySQL 8、React 19、TypeScript、Ant Design 5、Ant Design ProTable、Vitest、Docker Compose、Chrome。

---

## 实施边界

本计划只实现简易入驻闭环。申请字段固定为 `merchant_name` 和 `contact_phone`；不增加资质上传、营业执照、经营类目、申请撤回、短信供应商和多商家切换。

保持并补齐以下 API 契约：

```text
POST /api/merchant/apply
GET  /api/merchant/application
GET  /api/admin/merchants/applications
POST /api/admin/merchants/applications/audit
```

## 文件结构

### 后端修改

- `app/auth/api/internal/config/config.go`：声明调试验证码开关。
- `app/auth/api/internal/logic/auth/sendcodelogic.go`：按开关决定是否返回 `debug_code`。
- `app/auth/api/internal/logic/auth/sendcodelogic_test.go`：覆盖默认隐藏和本地开启。
- `app/auth/api/internal/logic/auth/registerlogic_test.go`：显式开启依赖验证码的测试配置。
- `app/auth/api/internal/logic/auth/logincodelogic_test.go`：显式开启依赖验证码的测试配置。
- `app/auth/api/internal/logic/auth/resetpasswordlogic_test.go`：显式开启依赖验证码的测试配置。
- `app/auth/api/etc/auth-api.yaml`：默认关闭调试验证码。
- `deploy/config/auth-api.yaml`：本地 Compose 开启调试验证码。
- `app/gateway/hertz/internal/handler/types.go`：声明申请详情、本人状态和管理员列表类型。
- `app/gateway/hertz/internal/handler/merchant_profile.go`：本人申请查询、申请校验和重复提交收敛。
- `app/gateway/hertz/internal/handler/admin_merchant_apply.go`：分页筛选、驳回原因校验和审核冲突处理。
- `app/gateway/hertz/internal/handler/routes.go`：注册本人申请查询路由。
- `app/gateway/hertz/internal/handler/routes_test.go`：断言新增路由存在。
- `app/gateway/hertz/internal/handler/merchant_application_test.go`：覆盖申请查询、重复申请、已有绑定和审核规则。

### 前端修改

- `frontend/packages/shared/src/types.ts`：共享验证码和商家申请类型。
- `frontend/packages/merchant/src/pages/LoginPage.tsx`：登录/注册双标签和调试验证码展示。
- `frontend/packages/merchant/src/pages/LoginPage.test.tsx`：注册与自动登录行为。
- `frontend/packages/merchant/src/components/MerchantOnboarding.tsx`：申请表和状态页。
- `frontend/packages/merchant/src/components/MerchantOnboarding.test.tsx`：无申请、待审核、驳回重提和停用展示。
- `frontend/packages/merchant/src/components/MerchantGuard.tsx`：完整入驻状态机。
- `frontend/packages/merchant/src/components/MerchantGuard.test.tsx`：状态分流测试。
- `frontend/packages/admin/src/pages/MerchantApplicationsPage.tsx`：管理员申请列表和审核操作。
- `frontend/packages/admin/src/pages/MerchantApplicationsPage.test.tsx`：筛选、通过、驳回和重复操作测试。
- `frontend/packages/admin/src/App.tsx`：注册管理员菜单和页面。
- `frontend/packages/admin/src/App.test.tsx`：断言菜单入口。
- `app/entry/api/internal/handler/web/admin.html`：由前端构建脚本重新生成。
- `app/entry/api/internal/handler/web/merchant.html`：由前端构建脚本重新生成。

### 配置与验收

- `deploy/docker-compose.yml`：无需新增服务；重建 `auth-api` 与 `hertz-gateway` 使用现有配置挂载和静态资源打包。
- `docs/superpowers/specs/2026-07-13-merchant-onboarding-flow-design.md`：验收时只对照，不再扩张范围。

---

### Task 1: 安全控制调试验证码

**Files:**
- Modify: `app/auth/api/internal/config/config.go`
- Modify: `app/auth/api/internal/logic/auth/sendcodelogic.go`
- Modify: `app/auth/api/internal/logic/auth/sendcodelogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/registerlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/logincodelogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/resetpasswordlogic_test.go`
- Modify: `app/auth/api/etc/auth-api.yaml`
- Modify: `deploy/config/auth-api.yaml`

- [ ] **Step 1: 将发送验证码成功测试改成默认不泄露**

把 `TestSendCodeLogic_Send_Success` 的断言改为：

```go
if resp.DebugCode != "" {
	t.Fatalf("debug code must be hidden by default, got %q", resp.DebugCode)
}
```

新增开启开关的测试：

```go
func TestSendCodeLogic_Send_ExposesDebugCodeWhenEnabled(t *testing.T) {
	svcCtx := svc.NewServiceContext(config.Config{
		JwtAuthSecret:          "test-auth-jwt-secret",
		JwtExpireSeconds:       600,
		DemoPassword:           "pwd",
		RefreshTokenTTLSeconds: 3600,
		CodeTTLSeconds:         300,
		ExposeDebugCode:        true,
	})

	resp, err := NewSendCodeLogic(context.Background(), svcCtx).Send(&types.SendCodeReq{
		Phone: "13800138000",
		Scene: "register",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DebugCode == "" {
		t.Fatal("expected debug code when ExposeDebugCode is enabled")
	}
}
```

- [ ] **Step 2: 运行测试确认默认隐藏测试失败**

Run:

```bash
go test ./app/auth/api/internal/logic/auth -run 'TestSendCodeLogic_Send_(Success|ExposesDebugCodeWhenEnabled)$' -count=1
```

Expected: `TestSendCodeLogic_Send_Success` 失败，因为当前实现始终返回验证码。

- [ ] **Step 3: 增加配置并保护响应字段**

在 `config.Config` 增加：

```go
ExposeDebugCode bool
```

在 `SendCodeLogic.Send` 构造响应前增加：

```go
debugCode := ""
if l.svcCtx.Config.ExposeDebugCode {
	debugCode = code
}
```

响应改为：

```go
return &types.SendCodeResp{
	Sent:      true,
	ExpiresAt: expiresAt.Unix(),
	DebugCode: debugCode,
}, nil
```

- [ ] **Step 4: 修正依赖验证码返回值的现有测试**

在下列测试中，凡是通过 `codeResp.DebugCode` 完成后续注册、验证码登录或重置密码的 `config.Config`，显式加入：

```go
ExposeDebugCode: true,
```

涉及文件：

```text
app/auth/api/internal/logic/auth/sendcodelogic_test.go
app/auth/api/internal/logic/auth/registerlogic_test.go
app/auth/api/internal/logic/auth/logincodelogic_test.go
app/auth/api/internal/logic/auth/resetpasswordlogic_test.go
```

不读取 `DebugCode` 的限流和审计测试保持默认关闭，用来防止测试配置掩盖泄露。

- [ ] **Step 5: 配置生产默认关闭、本地 Compose 开启**

在 `app/auth/api/etc/auth-api.yaml` 添加：

```yaml
ExposeDebugCode: false
```

在 `deploy/config/auth-api.yaml` 添加：

```yaml
ExposeDebugCode: true
```

不使用环境变量插值，确保缺少环境变量时不会把生产配置意外变成开启状态。

- [ ] **Step 6: 运行认证服务测试**

Run:

```bash
go test ./app/auth/api/internal/logic/auth ./app/auth/api/internal/svc -count=1
```

Expected: 两个包均输出 `ok`。

- [ ] **Step 7: 提交认证安全改动**

```bash
git add app/auth/api/internal/config/config.go \
  app/auth/api/internal/logic/auth/sendcodelogic.go \
  app/auth/api/internal/logic/auth/*logic_test.go \
  app/auth/api/etc/auth-api.yaml deploy/config/auth-api.yaml
git commit -m "feat: 安全展示本地注册验证码"
```

---

### Task 2: 增加本人申请查询并收紧申请规则

**Files:**
- Modify: `app/gateway/hertz/internal/handler/types.go`
- Modify: `app/gateway/hertz/internal/handler/merchant_profile.go`
- Modify: `app/gateway/hertz/internal/handler/routes.go`
- Modify: `app/gateway/hertz/internal/handler/routes_test.go`
- Create: `app/gateway/hertz/internal/handler/merchant_application_test.go`

- [ ] **Step 1: 先写申请类型和查询行为测试**

创建 `merchant_application_test.go`，使用 `sqlmock` 覆盖最近一次申请：

```go
func TestLoadLatestMerchantApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil { t.Fatal(err) }
	defer db.Close()

	mock.ExpectQuery("SELECT id, merchant_name, contact_phone, status").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_name", "contact_phone", "status", "merchant_id",
			"audit_remark", "create_time", "audit_time",
		}).AddRow(12, "测试商店", "13800000003", 2, 0, "名称不清晰", "2026-07-13 15:00:00", "2026-07-13 16:00:00"))

	got, err := loadLatestMerchantApplication(context.Background(), db, 1001)
	if err != nil { t.Fatal(err) }
	if got.Application == nil || got.Application.ApplyID != 12 || got.Application.StatusText != "rejected" {
		t.Fatalf("unexpected application: %#v", got.Application)
	}
}
```

再增加无记录测试，令查询返回 `sql.ErrNoRows`，断言 `Application == nil` 且 `err == nil`。

- [ ] **Step 2: 增加路由测试并确认失败**

在 `TestMerchantPageAndImageUploadRoutesAreIndependent` 的 `want` 中加入：

```go
"GET /api/merchant/application": false,
```

Run:

```bash
go test ./app/gateway/hertz/internal/handler -run 'TestLoadLatestMerchantApplication|TestMerchantPageAndImageUploadRoutesAreIndependent' -count=1
```

Expected: 编译失败或路由断言失败，因为类型、查询函数和路由尚未实现。

- [ ] **Step 3: 声明共享的后端响应类型**

在 `types.go` 的 `MerchantApplyResp` 后增加：

```go
type MerchantApplicationItem struct {
	ApplyID      int64  `json:"apply_id"`
	MerchantName string `json:"merchant_name"`
	ContactPhone string `json:"contact_phone"`
	Status       int64  `json:"status"`
	StatusText   string `json:"status_text"`
	MerchantID   int64  `json:"merchant_id"`
	AuditRemark  string `json:"audit_remark"`
	CreateTime   string `json:"create_time"`
	AuditTime    string `json:"audit_time"`
}

type MerchantApplicationResp struct {
	Application *MerchantApplicationItem `json:"application"`
}
```

在 `merchant_profile.go` 增加唯一的状态文本函数：

```go
func merchantApplicationStatusText(status int64) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "approved"
	case 2:
		return "rejected"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 4: 实现本人最近申请查询**

新增 `loadLatestMerchantApplication`，SQL 固定按用户查询，不接受外部用户 ID：

```go
func loadLatestMerchantApplication(ctx context.Context, db *sql.DB, userID int64) (MerchantApplicationResp, error) {
	row := db.QueryRowContext(ctx, `
SELECT id, merchant_name, contact_phone, status, merchant_id, audit_remark,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE user_id = ?
ORDER BY id DESC
LIMIT 1`, userID)

	item := MerchantApplicationItem{}
	if err := row.Scan(&item.ApplyID, &item.MerchantName, &item.ContactPhone, &item.Status,
		&item.MerchantID, &item.AuditRemark, &item.CreateTime, &item.AuditTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MerchantApplicationResp{Application: nil}, nil
		}
		return MerchantApplicationResp{}, err
	}
	item.StatusText = merchantApplicationStatusText(item.Status)
	return MerchantApplicationResp{Application: &item}, nil
}
```

新增 `MerchantApplicationHandler`：读取 JWT 身份、打开订单数据源、调用该函数并用 `ok` 返回。

- [ ] **Step 5: 注册本人申请查询路由**

在商家路由组加入：

```go
h.GET("/api/merchant/application", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), MerchantApplicationHandler(svcCtx))
```

兼容网关前缀的路由区域同时加入：

```go
h.GET("/api/gateway/merchant/application", middleware.RequireUser(svcCtx.Config.JwtAuthSecret), MerchantApplicationHandler(svcCtx))
```

- [ ] **Step 6: 增加申请输入校验**

在 `MerchantApplyCreateHandler` 中保留名称 trim，并增加：

```go
if req.ContactPhone != "" && !merchantContactPhonePattern.MatchString(req.ContactPhone) {
	fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "contact_phone is invalid"))
	return
}
```

将正则提升为包级变量，避免每次请求重复编译：

```go
var merchantContactPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
```

- [ ] **Step 7: 阻止已有有效商家的账号再次申请**

在创建申请前查询：

```sql
SELECT COUNT(*)
FROM merchant_user mu
JOIN merchant m ON m.id = mu.merchant_id
WHERE mu.user_id = ? AND mu.status = 1 AND m.status = 1
```

计数大于零时返回 HTTP `409` 和 `CodeConflict`，消息为 `active merchant already exists`。待审核申请仍返回原 `apply_id` 和 `pending`，驳回后允许插入新记录。

- [ ] **Step 8: 让重复待审核提交在事务内收敛**

把 `createMerchantApply` 改为事务实现：

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil { return MerchantApplyResp{}, err }
defer tx.Rollback()

var applyID int64
err = tx.QueryRowContext(ctx, `
SELECT id FROM merchant_apply
WHERE user_id = ? AND status = 0
ORDER BY id DESC LIMIT 1 FOR UPDATE`, userID).Scan(&applyID)
if err == nil {
	return MerchantApplyResp{ApplyID: applyID, Status: "pending"}, tx.Commit()
}
if !errors.Is(err, sql.ErrNoRows) { return MerchantApplyResp{}, err }

result, err := tx.ExecContext(ctx,
	"INSERT INTO merchant_apply (user_id, merchant_name, contact_phone, status) VALUES (?, ?, ?, 0)",
	userID, req.MerchantName, req.ContactPhone)
if err != nil { return MerchantApplyResp{}, err }
applyID, _ = result.LastInsertId()
if err := tx.Commit(); err != nil { return MerchantApplyResp{}, err }
return MerchantApplyResp{ApplyID: applyID, Status: "pending"}, nil
```

该查询必须保留 `FOR UPDATE`，并依赖现有 `(user_id, status)` InnoDB 索引锁定目标范围；不要把查询和插入拆回两个独立事务，也不要新增业务表或覆盖历史申请。

- [ ] **Step 9: 运行 Hertz 申请测试**

Run:

```bash
go test ./app/gateway/hertz/internal/handler -run 'TestLoadLatestMerchantApplication|TestMerchantPageAndImageUploadRoutesAreIndependent' -count=1
```

Expected: 所有目标测试输出 `PASS`。

- [ ] **Step 10: 提交本人申请接口**

```bash
git add app/gateway/hertz/internal/handler/types.go \
  app/gateway/hertz/internal/handler/merchant_profile.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go \
  app/gateway/hertz/internal/handler/merchant_application_test.go
git commit -m "feat: 补齐商家入驻状态接口"
```

---

### Task 3: 完成管理员申请分页与审核约束

**Files:**
- Modify: `app/gateway/hertz/internal/handler/types.go`
- Modify: `app/gateway/hertz/internal/handler/admin_merchant_apply.go`
- Modify: `app/gateway/hertz/internal/handler/merchant_application_test.go`

- [ ] **Step 1: 增加分页与驳回校验测试**

在测试文件增加纯解析测试，构造 Hertz `RequestContext` 并设置查询字符串：

```go
func TestAdminMerchantApplicationQueryNormalizesPagination(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?status=0&page=2&page_size=500")
	req, err := adminMerchantApplicationQueryFromRequest(c)
	if err != nil { t.Fatal(err) }
	if req.Status != 0 || req.Page != 2 || req.PageSize != 100 {
		t.Fatalf("unexpected query: %#v", req)
	}
}
```

再为审核输入提取 `validateAdminMerchantAuditRequest`，测试 `Approve=false` 且空备注返回错误，`Approve=true` 且空备注通过。

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./app/gateway/hertz/internal/handler -run 'TestAdminMerchantApplicationQuery|TestValidateAdminMerchantAuditRequest' -count=1
```

Expected: 编译失败，因为解析和校验函数尚不存在。

- [ ] **Step 3: 声明管理员列表类型**

在 `types.go` 增加：

```go
type AdminMerchantApplicationListReq struct {
	Status   int64 `json:"status"`
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
}

type AdminMerchantApplicationItem struct {
	MerchantApplicationItem
	UserID     int64 `json:"user_id"`
	OperatorID int64 `json:"operator_id"`
}

type AdminMerchantApplicationListResp struct {
	Items    []AdminMerchantApplicationItem `json:"items"`
	Total    int64                          `json:"total"`
	Page     int64                          `json:"page"`
	PageSize int64                          `json:"page_size"`
}
```

- [ ] **Step 4: 解析并规范化管理员查询参数**

实现：

```go
func adminMerchantApplicationQueryFromRequest(c *app.RequestContext) (AdminMerchantApplicationListReq, error) {
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil || status < -1 || status > 2 {
		return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be -1, 0, 1 or 2")
	}
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil { return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric") }
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil { return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric") }
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }
	if pageSize > 100 { pageSize = 100 }
	return AdminMerchantApplicationListReq{Status: status, Page: page, PageSize: pageSize}, nil
}
```

- [ ] **Step 5: 将固定 100 条查询改成 count + 分页**

使用同一个 `where` 和参数集合分别执行：

```sql
SELECT COUNT(*) FROM merchant_apply WHERE 1=1 [AND status = ?]
```

以及：

```sql
SELECT id, user_id, merchant_name, contact_phone, status, merchant_id,
       audit_remark, operator_id,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE 1=1 [AND status = ?]
ORDER BY CASE WHEN status = 0 THEN 0 ELSE 1 END, id DESC
LIMIT ? OFFSET ?
```

响应必须包含 `items`、`total`、`page` 和 `page_size`，每条记录用 `merchantApplicationStatusText` 生成 `status_text`。

- [ ] **Step 6: 服务端强制驳回原因**

实现并在 handler 解码后调用：

```go
func validateAdminMerchantAuditRequest(req adminMerchantApplyAuditReq) error {
	if req.ApplyID <= 0 {
		return apperror.New(apperror.CodeInvalidArgument, "apply_id is required")
	}
	if !req.Approve && strings.TrimSpace(req.Remark) == "" {
		return apperror.New(apperror.CodeInvalidArgument, "remark is required when rejecting")
	}
	return nil
}
```

校验失败返回 HTTP `400`；行锁查询到非待审核状态继续返回 HTTP `409`。

- [ ] **Step 7: 运行管理员申请测试与 Hertz 全包测试**

Run:

```bash
go test ./app/gateway/hertz/internal/handler -count=1
```

Expected: handler 包所有测试输出 `ok`。

- [ ] **Step 8: 提交管理员审核后端**

```bash
git add app/gateway/hertz/internal/handler/types.go \
  app/gateway/hertz/internal/handler/admin_merchant_apply.go \
  app/gateway/hertz/internal/handler/merchant_application_test.go
git commit -m "feat: 完善商家入驻审核接口"
```

---

### Task 4: 增加前端共享入驻类型

**Files:**
- Modify: `frontend/packages/shared/src/types.ts`

- [ ] **Step 1: 声明验证码响应**

在认证类型区域增加：

```ts
export interface SendCodeResp {
  sent: boolean;
  expires_at: number;
  debug_code?: string;
}
```

- [ ] **Step 2: 声明申请与审核类型**

在 `MerchantMeResp` 后增加：

```ts
export type MerchantApplicationStatus = 0 | 1 | 2;
export type MerchantApplicationStatusText = 'pending' | 'approved' | 'rejected';

export interface MerchantApplicationItem {
  apply_id: number;
  merchant_name: string;
  contact_phone: string;
  status: MerchantApplicationStatus;
  status_text: MerchantApplicationStatusText;
  merchant_id: number;
  audit_remark: string;
  create_time: string;
  audit_time: string;
}

export interface MerchantApplicationResp {
  application: MerchantApplicationItem | null;
}

export interface MerchantApplyResp {
  apply_id: number;
  status: 'pending';
}

export interface AdminMerchantApplicationItem extends MerchantApplicationItem {
  user_id: number;
  operator_id: number;
}

export interface AdminMerchantApplicationListResp {
  items: AdminMerchantApplicationItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface AdminMerchantAuditResp {
  apply_id: number;
  merchant_id: number;
  status: MerchantApplicationStatus;
}
```

- [ ] **Step 3: 运行共享包和 TypeScript 构建检查**

Run:

```bash
cd frontend
npm run test -w packages/shared -- --run
npm run build:merchant
npm run build:admin
```

Expected: shared 测试通过，merchant 和 admin 构建成功。

- [ ] **Step 4: 提交共享类型**

```bash
git add frontend/packages/shared/src/types.ts
git commit -m "feat: 统一商家入驻前端类型"
```

---

### Task 5: 在商家入口完成注册与自动登录

**Files:**
- Modify: `frontend/packages/merchant/src/pages/LoginPage.tsx`
- Create: `frontend/packages/merchant/src/pages/LoginPage.test.tsx`

- [ ] **Step 1: 写发送验证码与注册成功测试**

测试 mock `fetch` 的三次调用：发送验证码、注册、自动保存令牌。核心断言：

```tsx
expect(await screen.findByText('演示验证码：654321')).toBeInTheDocument();
expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({
  phone: '13800000003',
  scene: 'register',
});
expect(JSON.parse(String(fetchMock.mock.calls[1][1]?.body))).toEqual({
  phone: '13800000003',
  password: 'merchant-pass',
  code: '654321',
  display_name: '',
});
expect(localStorage.getItem('fm_token')).toBe('merchant-access');
expect(onLogin).toHaveBeenCalledOnce();
```

发送验证码响应使用：

```json
{"code":"OK","data":{"sent":true,"expires_at":1783940400,"debug_code":"654321"}}
```

注册响应使用 `LoginResp` 字段，并断言没有 `debug_code` 时页面不渲染演示验证码区域。

- [ ] **Step 2: 运行商家登录页测试确认失败**

Run:

```bash
cd frontend
npm run test -w packages/merchant -- --run src/pages/LoginPage.test.tsx
```

Expected: 找不到注册标签或“发送验证码”按钮。

- [ ] **Step 3: 将登录页改成双标签认证页**

保留当前登录逻辑，增加：

```ts
type AuthTab = 'login' | 'register';
type RegisterValues = { phone: string; code: string; password: string };
```

页面使用 Ant Design `Tabs`：

```tsx
<Tabs
  activeKey={tab}
  onChange={(key) => setTab(key as AuthTab)}
  items={[
    { key: 'login', label: '登录', children: loginForm },
    { key: 'register', label: '注册', children: registerForm },
  ]}
/>
```

注册表单字段 `phone`、`code`、`password` 均必填，手机号规则为 `/^1[3-9]\d{9}$/`，密码最少 6 位。

- [ ] **Step 4: 实现发送验证码和冷却倒计时**

发送逻辑：

```ts
const response = await api<SendCodeResp>('/api/auth/code/send', {
  method: 'POST',
  jsonBody: { phone, scene: 'register' },
});
if (!response.ok) {
  message.error(response.status === 429 ? '验证码发送过于频繁，请稍后再试' : '验证码发送失败');
  return;
}
setDebugCode(response.data.debug_code || '');
setCooldown(60);
```

用 `useEffect` 每秒递减 `cooldown`，组件卸载时清理定时器。按钮文字为 `重新发送（${cooldown}s）`，倒计时期间禁用。

- [ ] **Step 5: 实现注册成功自动登录**

注册逻辑：

```ts
const response = await api<LoginResp>('/api/auth/register', {
  method: 'POST',
  jsonBody: { ...values, display_name: '' },
});
if (!response.ok || !response.data.access_token) {
  message.error(response.status === 409 ? '该手机号已经注册，请直接登录' : '注册失败，请检查验证码和填写内容');
  return;
}
setToken(response.data.access_token);
if (response.data.refresh_token) setRefreshToken(response.data.refresh_token);
message.success('注册成功');
onLogin();
```

仅当 `debug_code` 非空时显示：

```tsx
<Alert type="info" showIcon message={`演示验证码：${debugCode}`} />
```

- [ ] **Step 6: 运行商家认证测试与构建**

Run:

```bash
cd frontend
npm run test -w packages/merchant -- --run src/pages/LoginPage.test.tsx
npm run build:merchant
```

Expected: 测试通过且 Vite 构建成功。

- [ ] **Step 7: 提交商家注册界面**

```bash
git add frontend/packages/merchant/src/pages/LoginPage.tsx \
  frontend/packages/merchant/src/pages/LoginPage.test.tsx
git commit -m "feat: 增加商家入口注册功能"
```

---

### Task 6: 实现商家入驻状态机和申请页面

**Files:**
- Create: `frontend/packages/merchant/src/components/MerchantOnboarding.tsx`
- Create: `frontend/packages/merchant/src/components/MerchantOnboarding.test.tsx`
- Modify: `frontend/packages/merchant/src/components/MerchantGuard.tsx`
- Modify: `frontend/packages/merchant/src/components/MerchantGuard.test.tsx`

- [ ] **Step 1: 为申请页面写四类状态测试**

测试必须覆盖：

```text
application=null                 -> 显示“申请入驻”表单
application.status=0             -> 显示“申请审核中”和申请编号
application.status=2             -> 显示审核备注并允许重新提交
disabled=true                    -> 显示“商家已停用”且没有申请按钮
```

驳回重提测试断言：

```tsx
expect(screen.getByDisplayValue('旧商店名称')).toBeInTheDocument();
expect(screen.getByDisplayValue('13800000003')).toBeInTheDocument();
await user.clear(screen.getByLabelText('商家名称'));
await user.type(screen.getByLabelText('商家名称'), '新商店名称');
await user.click(screen.getByRole('button', { name: '重新提交申请' }));
expect(mocks.authed).toHaveBeenCalledWith('/api/merchant/apply', expect.objectContaining({
  method: 'POST',
  jsonBody: { merchant_name: '新商店名称', contact_phone: '13800000003' },
}));
```

- [ ] **Step 2: 运行申请页测试确认失败**

Run:

```bash
cd frontend
npm run test -w packages/merchant -- --run src/components/MerchantOnboarding.test.tsx
```

Expected: 模块不存在。

- [ ] **Step 3: 创建 MerchantOnboarding 的明确接口**

```ts
interface MerchantOnboardingProps {
  application: MerchantApplicationItem | null;
  disabled: boolean;
  initializing: boolean;
  onRefresh: () => void;
  onSwitchAccount: () => void;
}
```

无申请或已驳回时渲染 `Form`。首次无申请时调用 `authed<MeResp>('/api/auth/me')`，把返回手机号写入 `contact_phone`；驳回时优先使用申请中的原值。

- [ ] **Step 4: 实现提交和状态卡片**

提交使用：

```ts
const response = await authed<MerchantApplyResp>('/api/merchant/apply', {
  method: 'POST',
  jsonBody: {
    merchant_name: values.merchant_name.trim(),
    contact_phone: values.contact_phone.trim(),
  },
});
if (!response.ok) {
  message.error(response.status === 409 ? '当前账号已经绑定商家' : '申请提交失败，请稍后重试');
  return;
}
message.success('入驻申请已提交');
onRefresh();
```

待审核状态显示申请编号、商家名称、联系电话和申请时间；驳回状态显示审核原因和审核时间；初始化状态显示 `Spin` 与“刷新商家身份”；停用状态只显示联系管理员和切换账号。

- [ ] **Step 5: 先扩充 MerchantGuard 状态测试**

将旧的 `unbound` 测试拆成：

1. `/api/merchant/me` 返回空数组、`/api/merchant/application` 返回 `null`，断言出现申请表；
2. 返回待审核申请，断言出现“申请审核中”；
3. 返回停用商家，断言出现“商家已停用”且不请求申请接口；
4. 返回有效商家，断言经营内容出现；
5. 本人申请查询失败，断言出现“入驻状态加载失败”和重试按钮。

- [ ] **Step 6: 实现守卫分流函数**

守卫状态声明：

```ts
type GuardState =
  | 'loading'
  | 'login'
  | 'onboarding'
  | 'approved_initializing'
  | 'disabled'
  | 'authorized'
  | 'error';
```

验证顺序固定为：

```ts
const merchantResponse = await authed<MerchantMeResp>('/api/merchant/me');
if (!merchantResponse.ok) { setState('error'); return; }
const merchants = merchantResponse.data.items || [];
if (merchants.some((item) => item.status === 1)) { setState('authorized'); return; }
if (merchants.length > 0) { setState('disabled'); return; }

const applicationResponse = await authed<MerchantApplicationResp>('/api/merchant/application');
if (!applicationResponse.ok) { setState('error'); return; }
setApplication(applicationResponse.data.application);
setState(applicationResponse.data.application?.status === 1 ? 'approved_initializing' : 'onboarding');
```

`verify` 使用 `useCallback`，避免 effect 和子组件回调持有旧闭包。

- [ ] **Step 7: 清理账号切换状态**

实现统一方法：

```ts
const switchAccount = () => {
  clearAuth();
  setApplication(null);
  setState('login');
};
```

所有入驻状态都使用该方法，不直接操作 localStorage。

- [ ] **Step 8: 运行商家组件全量测试与构建**

Run:

```bash
cd frontend
npm run test -w packages/merchant -- --run
npm run build:merchant
```

Expected: merchant 包测试全部通过，构建成功。

- [ ] **Step 9: 提交入驻状态机**

```bash
git add frontend/packages/merchant/src/components/MerchantOnboarding.tsx \
  frontend/packages/merchant/src/components/MerchantOnboarding.test.tsx \
  frontend/packages/merchant/src/components/MerchantGuard.tsx \
  frontend/packages/merchant/src/components/MerchantGuard.test.tsx
git commit -m "feat: 完成商家入驻状态闭环"
```

---

### Task 7: 增加管理员商家入驻审核页面

**Files:**
- Create: `frontend/packages/admin/src/pages/MerchantApplicationsPage.tsx`
- Create: `frontend/packages/admin/src/pages/MerchantApplicationsPage.test.tsx`
- Modify: `frontend/packages/admin/src/App.tsx`
- Create or Modify: `frontend/packages/admin/src/App.test.tsx`

- [ ] **Step 1: 写管理员页面加载与审核测试**

mock `authed`，列表请求返回一条待审核记录。断言：

```tsx
expect(await screen.findByText('测试商店')).toBeInTheDocument();
expect(screen.getByRole('button', { name: '通过' })).toBeInTheDocument();
expect(screen.getByRole('button', { name: '驳回' })).toBeInTheDocument();
```

通过操作断言请求：

```ts
expect(mocks.authed).toHaveBeenCalledWith('/api/admin/merchants/applications/audit', {
  method: 'POST',
  jsonBody: { apply_id: 12, approve: true, remark: '' },
});
```

驳回测试先不填写备注，断言没有调用审核接口；填写“名称不符合规范”后断言 `approve:false` 和该备注被提交。

- [ ] **Step 2: 写菜单路由测试并确认失败**

在 `App.test.tsx` mock 页面后渲染 `App`，断言：

```tsx
expect(await screen.findByText('商家入驻')).toBeInTheDocument();
```

Run:

```bash
cd frontend
npm run test -w packages/admin -- --run src/pages/MerchantApplicationsPage.test.tsx src/App.test.tsx
```

Expected: 页面模块或菜单项不存在。

- [ ] **Step 3: 创建 ProTable 申请列表**

页面使用 `ProTable<AdminMerchantApplicationItem>`，状态映射固定为：

```ts
const statusValueEnum = {
  '-1': { text: '全部' },
  '0': { text: '待审核', status: 'Processing' },
  '1': { text: '已通过', status: 'Success' },
  '2': { text: '已驳回', status: 'Error' },
};
```

`request` 构造：

```ts
const query = new URLSearchParams();
query.set('page', String(params.current || 1));
query.set('page_size', String(params.pageSize || 20));
if (params.status !== undefined && String(params.status) !== '-1') {
  query.set('status', String(params.status));
}
const response = await authed<AdminMerchantApplicationListResp>(
  `/api/admin/merchants/applications?${query}`,
);
return {
  data: response.ok ? response.data.items || [] : [],
  total: response.ok ? response.data.total : 0,
  success: response.ok,
};
```

列包含申请编号、用户 ID、商家名称、联系电话、状态、申请时间、审核备注、审核人、审核时间和操作。

- [ ] **Step 4: 实现通过与驳回弹窗**

统一审核函数：

```ts
const audit = async (approve: boolean, remark: string) => {
  if (!selected) return;
  const response = await authed<AdminMerchantAuditResp>('/api/admin/merchants/applications/audit', {
    method: 'POST',
    jsonBody: { apply_id: selected.apply_id, approve, remark: remark.trim() },
  });
  if (!response.ok) {
    message.error(response.status === 409 ? '该申请已经处理，列表已刷新' : '审核提交失败');
    actionRef.current?.reload();
    return;
  }
  message.success(approve ? '申请已通过' : '申请已驳回');
  setSelected(null);
  setAuditMode(null);
  form.resetFields();
  actionRef.current?.reload();
};
```

驳回模式的 `remark` 使用必填规则；通过模式显示可选备注。只有 `status === 0` 的行渲染操作按钮。

- [ ] **Step 5: 接入管理员菜单**

在 `App.tsx`：

```tsx
import { AuditOutlined } from '@ant-design/icons';
import MerchantApplicationsPage from './pages/MerchantApplicationsPage';
```

路由映射增加：

```tsx
'/admin/merchant-applications': MerchantApplicationsPage,
```

菜单增加：

```tsx
{ path: '/admin/merchant-applications', name: '商家入驻', icon: <AuditOutlined /> },
```

不引入新的路由框架，沿用当前 `routeMap` 和 `setPathname`。

- [ ] **Step 6: 运行管理员测试与构建**

Run:

```bash
cd frontend
npm run test -w packages/admin -- --run
npm run build:admin
```

Expected: admin 包测试全部通过，Vite 构建成功。

- [ ] **Step 7: 提交管理员审核界面**

```bash
git add frontend/packages/admin/src/pages/MerchantApplicationsPage.tsx \
  frontend/packages/admin/src/pages/MerchantApplicationsPage.test.tsx \
  frontend/packages/admin/src/App.tsx frontend/packages/admin/src/App.test.tsx
git commit -m "feat: 增加商家入驻审核页面"
```

---

### Task 8: 生成静态资源并完成分层验证

**Files:**
- Modify: `app/entry/api/internal/handler/web/admin.html`
- Modify: `app/entry/api/internal/handler/web/merchant.html`

- [ ] **Step 1: 运行 Go 格式化和目标测试**

Run:

```bash
gofmt -w app/auth/api/internal/config/config.go \
  app/auth/api/internal/logic/auth/sendcodelogic.go \
  app/auth/api/internal/logic/auth/*logic_test.go \
  app/gateway/hertz/internal/handler/types.go \
  app/gateway/hertz/internal/handler/merchant_profile.go \
  app/gateway/hertz/internal/handler/admin_merchant_apply.go \
  app/gateway/hertz/internal/handler/routes.go \
  app/gateway/hertz/internal/handler/routes_test.go \
  app/gateway/hertz/internal/handler/merchant_application_test.go
go test ./app/auth/api/internal/logic/auth ./app/auth/api/internal/svc -count=1
go test ./app/gateway/hertz/internal/handler -count=1
```

Expected: 所有目标 Go 包输出 `ok`。

- [ ] **Step 2: 运行前端目标测试**

Run:

```bash
cd frontend
npm run test -w packages/shared -- --run
npm run test -w packages/merchant -- --run
npm run test -w packages/admin -- --run
```

Expected: 三个工作区测试全部通过。

- [ ] **Step 3: 重新生成单文件静态页面**

Run:

```bash
cd frontend
npm run build
```

Expected: 输出包含：

```text
[build] Copied admin.html
[build] Copied merchant.html
[build] Done!
```

确认生成文件包含新文案：

```bash
grep -q '商家入驻' ../app/entry/api/internal/handler/web/admin.html
grep -q '发送验证码' ../app/entry/api/internal/handler/web/merchant.html
```

- [ ] **Step 4: 构建最小必要 Docker 镜像**

在 Windows PowerShell 调用项目现有构建脚本，只构建受影响服务；若脚本不支持服务过滤，则使用 Compose：

```powershell
wsl -d Ubuntu -- bash -lc "cd /home/mildred/code/flash-mall/.worktrees/desktop-launcher && docker compose -f deploy/docker-compose.yml build auth-api hertz-gateway"
```

Expected: 两个镜像成功构建，未重新构建 product、order、inventory 服务。

- [ ] **Step 5: 重建受影响容器并检查健康状态**

```bash
docker compose -f deploy/docker-compose.yml up -d --no-deps auth-api hertz-gateway
docker compose -f deploy/docker-compose.yml ps auth-api hertz-gateway mysql redis
curl -fsS http://127.0.0.1:8889/healthz
```

Expected: `auth-api` 和 `hertz-gateway` 为 running/healthy，健康接口返回成功。

- [ ] **Step 6: 用 API 验证调试验证码开关和申请状态**

本地发送验证码：

```bash
curl -fsS -X POST http://127.0.0.1:8889/api/auth/code/send \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13900000013","scene":"register"}'
```

Expected: 本地响应 `data.debug_code` 非空。关闭开关不泄露验证码的证据使用 Task 1 中 `TestSendCodeLogic_Send_Success` 的默认关闭测试，不在验收阶段改动运行配置。

- [ ] **Step 7: 用 Chrome 跑通批准链路**

使用真实浏览器按顺序执行：

```text
打开 /merchant
切换“注册”
输入未使用手机号并发送验证码
读取页面显示的动态验证码并注册
填写商家名称和联系电话并提交
刷新页面确认仍显示待审核
在独立浏览器上下文打开 /admin 并一键登录
进入“商家入驻”并通过该申请
返回商家上下文刷新审核状态
确认自动进入数据概览
```

注意 `/merchant` 与 `/admin` 同源并共享 localStorage，必须使用两个 Chrome 上下文，避免管理员令牌覆盖商家令牌。

- [ ] **Step 8: 用 Chrome 跑通驳回重提链路**

再注册一个新账号，提交申请后由管理员驳回并填写原因。商家端刷新后必须看到原因、旧资料回填和“重新提交申请”；修改名称重提后，管理员列表同时保留旧驳回记录与新待审核记录。

- [ ] **Step 9: 验证权限和停用边界**

通过浏览器网络面板或 curl 确认：

```text
普通用户请求 GET /api/admin/merchants/applications -> 403
未登录请求 GET /api/merchant/application -> 401
本人申请接口不存在 user_id 查询参数，也不会返回其他用户记录
停用商家登录 /merchant -> 显示停用状态，不显示申请表
```

- [ ] **Step 10: 检查差异并提交生成资源**

Run:

```bash
git diff --check
git status --short
git diff --stat
```

只应出现本计划列出的源文件、测试和两个生成 HTML。提交：

```bash
git add app/entry/api/internal/handler/web/admin.html \
  app/entry/api/internal/handler/web/merchant.html
git commit -m "build: 更新商家入驻前端资源"
```

---

### Task 9: 最终回归与交付检查

**Files:**
- Verify only; no source file should be created in this task.

- [ ] **Step 1: 运行完整但受控的项目检查**

Run:

```bash
go test ./app/auth/api/... ./app/gateway/hertz/... -count=1
cd frontend
npm run test -w packages/shared -- --run
npm run test -w packages/merchant -- --run
npm run test -w packages/admin -- --run
npm run test -w packages/shop -- --run
npm run build
```

Expected: Auth API、Hertz Gateway 和全部前端工作区通过；不运行与本次功能无关的压测、故障注入或全量 Docker 重建。

- [ ] **Step 2: 检查提交序列和工作区**

```bash
git log --oneline -8
git status --short --branch
```

Expected: 提交按“验证码安全 → 后端状态接口 → 审核后端 → 共享类型 → 商家注册 → 入驻状态机 → 管理员审核页 → 静态资源”排列，工作区干净。

- [ ] **Step 3: 记录最终验收证据**

最终汇报必须包含：

```text
后端测试命令及通过结果
前端测试与构建命令及通过结果
Docker 中 auth-api、hertz-gateway 的运行状态
Chrome 批准链路结果
Chrome 驳回重提链路结果
调试验证码关闭时不泄露的结果
当前分支和提交范围
```

不得仅以“代码已完成”代替运行证据，也不得在未验证时宣称流程已经闭环。

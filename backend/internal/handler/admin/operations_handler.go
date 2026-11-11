package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OperationsHandler deliberately exposes a separate, narrow API. Operators never
// pass adminAuth, and cannot reuse the unrestricted administrative handlers.
type OperationsHandler struct {
	users         service.AdminService
	subscriptions *service.SubscriptionService
	plans         *service.PaymentConfigService
}

func NewOperationsHandler(users *UserHandler, subscriptions *SubscriptionHandler, payment ...*PaymentHandler) *OperationsHandler {
	h := &OperationsHandler{users: users.adminService, subscriptions: subscriptions.subscriptionService}
	if len(payment) > 0 && payment[0] != nil {
		h.plans = payment[0].configService
	}
	return h
}

func OperationsRoleRequired(c *gin.Context) {
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || (role != service.RoleAdmin && role != service.RoleOperator) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "需要运营管理员权限"})
		return
	}
	c.Next()
}

// Only explicitly approved customer-facing products belong to this business.
// Historical private upstream groups must not be granted by an operator.
func operationsGroup(g *service.Group) bool { return service.IsCustomerPlanGroup(g) }

// Strict schemas also reject duplicate keys, case aliases and trailing JSON.
// This prevents hidden role/balance fields from bypassing the restricted forms.
func operationsBody(c *gin.Context, allowed []string, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	decoder := json.NewDecoder(c.Request.Body)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		response.BadRequest(c, "请求必须是 JSON 对象")
		return false
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			response.BadRequest(c, "无效的请求字段")
			return false
		}
		valid := false
		for _, name := range allowed {
			if name == key {
				valid = true
				break
			}
		}
		if _, duplicate := fields[key]; duplicate || !valid {
			response.BadRequest(c, "不允许的字段或重复字段："+key)
			return false
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil || string(value) == "null" {
			response.BadRequest(c, "字段值无效："+key)
			return false
		}
		fields[key] = value
	}
	if _, err = decoder.Token(); err != nil {
		response.BadRequest(c, "无效的 JSON 请求")
		return false
	}
	if _, err = decoder.Token(); err != io.EOF {
		response.BadRequest(c, "请求只能包含一个 JSON 对象")
		return false
	}
	data, _ := json.Marshal(fields)
	if err = json.Unmarshal(data, target); err != nil {
		response.BadRequest(c, "字段类型无效")
		return false
	}
	if err = service.ValidateOperationsInput(target); err != nil {
		response.BadRequest(c, err.Error())
		return false
	}
	return true
}

func operationsCustomer(u *service.User) gin.H {
	return gin.H{"id": u.ID, "email": u.Email, "username": u.Username, "notes": u.Notes, "role": u.Role, "status": u.Status, "balance": u.Balance, "concurrency": u.Concurrency}
}

func (h *OperationsHandler) customer(c *gin.Context) *service.User {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "客户编号无效")
		return nil
	}
	u, err := h.users.GetUser(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "客户不存在")
		return nil
	}
	if u.Role != service.RoleUser {
		response.Forbidden(c, "运营管理员只能操作普通客户")
		return nil
	}
	return u
}

func (h *OperationsHandler) Customers(c *gin.Context) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	include := false
	users, total, err := h.users.ListUsers(c.Request.Context(), page, size, service.UserListFilters{Role: service.RoleUser, Search: strings.TrimSpace(c.Query("search")), IncludeSubscriptions: &include}, "id", "desc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]gin.H, 0, len(users))
	for i := range users {
		if users[i].Role == service.RoleUser {
			out = append(out, operationsCustomer(&users[i]))
		}
	}
	response.Paginated(c, out, total, page, size)
}

func (h *OperationsHandler) Create(c *gin.Context) {
	var req service.OperationsCreateInput
	if !operationsBody(c, []string{"email", "password", "username", "notes"}, &req) {
		return
	}
	groups, err := h.users.GetAllGroups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowed := []int64{}
	for i := range groups {
		if operationsGroup(&groups[i]) && !isProNativeSeat(&groups[i]) {
			allowed = append(allowed, groups[i].ID)
		}
	}
	if len(allowed) == 0 {
		response.BadRequest(c, "尚未配置客户业务分组")
		return
	}
	zero := 0.0
	u, err := h.users.CreateUser(c.Request.Context(), &service.CreateUserInput{Email: req.Email, Password: req.Password, Username: req.Username, Notes: req.Notes, Role: service.RoleUser, Balance: &zero, Concurrency: 1, AllowedGroups: allowed, RestrictPublicGroups: true, ActorAdminID: getAdminIDFromContext(c)})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operationsCustomer(u))
}

func (h *OperationsHandler) Update(c *gin.Context) {
	u := h.customer(c)
	if u == nil {
		return
	}
	var req service.OperationsUpdateInput
	if !operationsBody(c, []string{"username", "notes", "status", "concurrency"}, &req) {
		return
	}
	u, err := h.users.UpdateUser(c.Request.Context(), u.ID, &service.UpdateUserInput{Username: req.Username, Notes: req.Notes, Status: req.Status, Concurrency: req.Concurrency, ActorAdminID: getAdminIDFromContext(c)})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operationsCustomer(u))
}

func (h *OperationsHandler) Recharge(c *gin.Context) {
	u := h.customer(c)
	if u == nil {
		return
	}
	var req service.OperationsRechargeInput
	if !operationsBody(c, []string{"amount", "notes"}, &req) {
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) < 16 || len(key) > 128 {
		response.BadRequest(c, "充值请求缺少有效的幂等编号")
		return
	}
	payload := struct {
		UserID int64
		Body   service.OperationsRechargeInput
	}{u.ID, req}
	executeOperationsIdempotent(c, "operations.customers.recharge", payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		updated, err := h.users.UpdateUserBalance(ctx, u.ID, req.Amount, "add", req.Notes)
		if err != nil {
			return nil, err
		}
		return operationsCustomer(updated), nil
	})
}

func (h *OperationsHandler) Groups(c *gin.Context) {
	groups, err := h.users.GetAllGroups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := []gin.H{}
	for i := range groups {
		g := &groups[i]
		if operationsGroup(g) && !isProNativeSeat(g) && g.SubscriptionType == service.SubscriptionTypeSubscription {
			out = append(out, gin.H{"id": g.ID, "name": g.Name, "daily_limit": g.DailyLimitUSD, "weekly_limit": g.WeeklyLimitUSD, "monthly_limit": g.MonthlyLimitUSD})
		}
	}
	response.Success(c, out)
}

func operationsSubscription(s *service.UserSubscription) gin.H {
	name := ""
	if s.Group != nil {
		name = s.Group.Name
	}
	return gin.H{"id": s.ID, "user_id": s.UserID, "group_id": s.GroupID, "group_name": name, "status": s.Status, "expires_at": s.ExpiresAt, "daily_usage": s.DailyUsageUSD, "weekly_usage": s.WeeklyUsageUSD, "monthly_usage": s.MonthlyUsageUSD, "notes": s.Notes}
}

func (h *OperationsHandler) Subscriptions(c *gin.Context) {
	u := h.customer(c)
	if u == nil {
		return
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	subs, p, err := h.subscriptions.List(c.Request.Context(), page, size, &u.ID, nil, "", "", "created_at", "desc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := []gin.H{}
	for i := range subs {
		if subs[i].UserID == u.ID {
			out = append(out, operationsSubscription(&subs[i]))
		}
	}
	response.PaginatedWithResult(c, out, toResponsePagination(p))
}

func (h *OperationsHandler) Assign(c *gin.Context) {
	u := h.customer(c)
	if u == nil {
		return
	}
	var req service.OperationsAssignInput
	if !operationsBody(c, []string{"group_id", "validity_days", "notes", "plan_id"}, &req) {
		return
	}
	if req.PlanID > 0 {
		plan, err := h.plans.GetPlan(c.Request.Context(), req.PlanID)
		if err != nil || !plan.ForSale || plan.GroupID != req.GroupID || plan.ValidityDays != req.ValidityDays || plan.ValidityUnit != "day" {
			response.BadRequest(c, "套餐商品已变更或下架，请刷新后重试")
			return
		}
	}
	g, err := h.users.GetGroup(c.Request.Context(), req.GroupID)
	if err != nil || !operationsGroup(g) || g.SubscriptionType != service.SubscriptionTypeSubscription {
		response.Forbidden(c, "只能开通客户共享月套餐")
		return
	}
	if isProNativeSeat(g) {
		response.Forbidden(c, "独享席位由系统自动分配，请选择“Claude 独享 Pro”商品")
		return
	}
	if g.Platform != service.PlatformAnthropic {
		ready, err := h.catalogReady(c.Request.Context(), g)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if !ready {
			response.BadRequest(c, "该模型上游尚未接入，暂不能开通套餐")
			return
		}
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) < 16 || len(key) > 128 {
		response.BadRequest(c, "开通请求缺少有效的幂等编号")
		return
	}
	payload := struct {
		UserID int64
		Body   service.OperationsAssignInput
	}{u.ID, req}
	executeOperationsIdempotent(c, "operations.subscriptions.assign", payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if isProNativeTemplate(g) {
			seatAllocationMu.Lock()
			defer seatAllocationMu.Unlock()
		}
		existing, _, err := h.subscriptions.List(ctx, 1, 100, &u.ID, nil, "active", "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		for i := range existing {
			other := &existing[i]
			if other.GroupID == req.GroupID || !other.IsActive() {
				continue
			}
			otherGroup, groupErr := h.users.GetGroup(ctx, other.GroupID)
			if groupErr != nil {
				return nil, groupErr
			}
			if isProNativeTemplate(g) && isProNativeSeat(otherGroup) {
				continue // assignProNative reports the precise "already holds a seat" error
			}
			if operationsGroup(otherGroup) && otherGroup.Platform == g.Platform {
				return nil, infraerrors.Conflict("ACTIVE_SHARED_PLAN", "客户已有其他有效共享套餐，请由系统管理员处理换档")
			}
		}
		input := service.AssignSubscriptionInput{UserID: u.ID, GroupID: req.GroupID, ValidityDays: req.ValidityDays, Notes: req.Notes, AssignedBy: getAdminIDFromContext(c)}
		var sub *service.UserSubscription
		if isProNativeTemplate(g) {
			sub, err = h.assignProNative(ctx, u.ID, input)
		} else {
			sub, err = h.subscriptions.AssignSubscription(ctx, &input)
		}
		if err != nil {
			return nil, err
		}
		return operationsSubscription(sub), nil
	})
}

func (h *OperationsHandler) Extend(c *gin.Context) {
	u := h.customer(c)
	if u == nil {
		return
	}
	id, err := strconv.ParseInt(c.Param("subscription_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "套餐编号无效")
		return
	}
	sub, err := h.subscriptions.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "套餐不存在")
		return
	}
	if sub.UserID != u.ID {
		response.Forbidden(c, "套餐不属于当前客户")
		return
	}
	g, err := h.users.GetGroup(c.Request.Context(), sub.GroupID)
	if err != nil || !operationsGroup(g) {
		response.Forbidden(c, "只能续期客户共享月套餐")
		return
	}
	var req service.OperationsExtendInput
	if !operationsBody(c, []string{"days"}, &req) {
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) < 16 || len(key) > 128 {
		response.BadRequest(c, "续期请求缺少有效的幂等编号")
		return
	}
	payload := struct {
		SubscriptionID int64
		Body           service.OperationsExtendInput
	}{id, req}
	executeOperationsIdempotent(c, "operations.subscriptions.extend", payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		updated, err := h.subscriptions.ExtendSubscription(ctx, id, req.Days)
		if err != nil {
			return nil, err
		}
		return operationsSubscription(updated), nil
	})
}

// Financial and entitlement writes must not run without duplicate protection.
func executeOperationsIdempotent(c *gin.Context, scope string, payload any, ttl time.Duration, execute func(context.Context) (any, error)) {
	if service.DefaultIdempotencyCoordinator() == nil {
		response.ErrorFrom(c, service.ErrIdempotencyStoreUnavail)
		return
	}
	executeAdminIdempotentJSON(c, scope, payload, ttl, execute)
}

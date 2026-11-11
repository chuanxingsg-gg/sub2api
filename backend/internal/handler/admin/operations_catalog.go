package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *OperationsHandler) catalogReady(ctx context.Context, g *service.Group) (bool, error) {
	_, count, err := h.users.ListAccounts(ctx, 1, 1, g.Platform, "", service.StatusActive, "", g.ID, "", "id", "asc")
	return count > 0, err
}

func (h *OperationsHandler) catalog(c *gin.Context, public bool) {
	plans, err := h.plans.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	groups, err := h.users.GetAllGroups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowed := map[int64]*service.Group{}
	ready := map[int64]bool{}
	groupRows := []gin.H{}
	for i := range groups {
		g := &groups[i]
		if !operationsGroup(g) || isProNativeSeat(g) {
			continue // seats are an internal allocation detail, never a catalog row
		}
		available, err := h.catalogReady(c.Request.Context(), g)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		seatsFree := 0
		if isProNativeTemplate(g) {
			free, err := h.freeSeats(c.Request.Context())
			if err != nil {
				response.ErrorFrom(c, err)
				return
			}
			seatsFree = len(free)
			available = seatsFree > 0
		}
		allowed[g.ID] = g
		ready[g.ID] = available
		family := map[string]string{service.PlatformAnthropic: "Claude", service.PlatformOpenAI: "GPT", service.PlatformGrok: "Grok", service.PlatformGemini: "Gemini"}[g.Platform]
		concurrency := 1
		if strings.HasSuffix(g.Name, "-Advanced") {
			concurrency = 2
		}
		row := gin.H{"id": g.ID, "name": g.Name, "family": family, "kind": g.SubscriptionType, "concurrency": concurrency, "daily_limit": g.DailyLimitUSD, "weekly_limit": g.WeeklyLimitUSD, "monthly_limit": g.MonthlyLimitUSD, "ready": available, "exclusive": isProNativeTemplate(g)}
		if isProNativeTemplate(g) && !public {
			row["seats_available"] = seatsFree // exact stock is for operators only
		}
		groupRows = append(groupRows, row)
	}
	rows := []gin.H{}
	for _, p := range plans {
		g := allowed[p.GroupID]
		if g == nil || g.SubscriptionType != service.SubscriptionTypeSubscription || (public && !p.ForSale) {
			continue
		}
		rows = append(rows, gin.H{"id": p.ID, "group_id": p.GroupID, "name": p.Name, "description": p.Description, "price": p.Price, "currency": "CNY", "validity_days": p.ValidityDays, "for_sale": p.ForSale, "sort_order": p.SortOrder, "ready": ready[p.GroupID]})
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"plans": rows, "groups": groupRows})
}
func (h *OperationsHandler) PublicCatalog(c *gin.Context)     { h.catalog(c, true) }
func (h *OperationsHandler) Plans(c *gin.Context)             { h.catalog(c, false) }
func (h *OperationsHandler) CreateCatalogPlan(c *gin.Context) { h.writeCatalogPlan(c, false) }
func (h *OperationsHandler) UpdateCatalogPlan(c *gin.Context) { h.writeCatalogPlan(c, true) }
func (h *OperationsHandler) writeCatalogPlan(c *gin.Context, update bool) {
	var req service.OperationsPlanInput
	if !operationsBody(c, []string{"group_id", "name", "description", "price", "validity_days", "for_sale", "sort_order"}, &req) {
		return
	}
	g, err := h.users.GetGroup(c.Request.Context(), req.GroupID)
	if err != nil || !operationsGroup(g) || isProNativeSeat(g) || g.SubscriptionType != service.SubscriptionTypeSubscription {
		response.Forbidden(c, "只能为客户业务套餐配置商品，不能使用历史、私有分组或独享席位")
		return
	}
	var id int64
	if update {
		id, err = strconv.ParseInt(c.Param("plan_id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "套餐编号无效")
			return
		}
		old, err := h.plans.GetPlan(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		oldGroup, err := h.users.GetGroup(c.Request.Context(), old.GroupID)
		if err != nil || !operationsGroup(oldGroup) {
			response.Forbidden(c, "不能编辑此套餐")
			return
		}
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) < 16 || len(key) > 128 {
		response.BadRequest(c, "保存请求缺少有效的幂等编号")
		return
	}
	payload := struct {
		ID   int64
		Body service.OperationsPlanInput
	}{id, req}
	executeOperationsIdempotent(c, "operations.catalog.save", payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		currency, unit := "CNY", "day"
		if update {
			return h.plans.UpdatePlan(ctx, id, service.UpdatePlanRequest{GroupID: &req.GroupID, Name: &req.Name, Description: &req.Description, Price: &req.Price, Currency: &currency, ValidityDays: &req.ValidityDays, ValidityUnit: &unit, ForSale: &req.ForSale, SortOrder: &req.SortOrder})
		}
		return h.plans.CreatePlan(ctx, service.CreatePlanRequest{GroupID: req.GroupID, Name: req.Name, Description: req.Description, Price: req.Price, Currency: currency, ValidityDays: req.ValidityDays, ValidityUnit: unit, ForSale: req.ForSale, SortOrder: req.SortOrder})
	})
}

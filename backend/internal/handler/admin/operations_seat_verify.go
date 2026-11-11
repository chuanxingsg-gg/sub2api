package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SeatVerifyHandler lets an administrator check that a Claude 独享 Pro seat works
// end to end for its customer: subscription, seat group, bound account, the
// customer's key, a real gateway call and the 5h / weekly quota read.
type SeatVerifyHandler struct {
	users         service.AdminService
	subscriptions *service.SubscriptionService
	usage         *service.AccountUsageService
	client        *http.Client
}

func NewSeatVerifyHandler(users *UserHandler, subscriptions *SubscriptionHandler, accounts *AccountHandler) *SeatVerifyHandler {
	h := &SeatVerifyHandler{client: &http.Client{Timeout: 70 * time.Second}}
	if users != nil {
		h.users = users.adminService
	}
	if subscriptions != nil {
		h.subscriptions = subscriptions.subscriptionService
	}
	if accounts != nil {
		h.usage = accounts.accountUsageService
	}
	return h
}

// Cheapest first. Only models with a configured price are offered.
var seatVerifyModels = []string{"claude-haiku-4-5-20251001", "claude-sonnet-5-5", "claude-opus-5-5"}

type seatCheck struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type seatKeyView struct {
	ID      int64
	Status  string
	GroupID int64
}

// evaluateSeatStructure is the pure, side-effect free part of the check.
func evaluateSeatStructure(sub *service.UserSubscription, group *service.Group, accounts []service.Account, keys []seatKeyView, now time.Time) []seatCheck {
	checks := []seatCheck{}
	add := func(key, label string, ok bool, detail string) {
		checks = append(checks, seatCheck{key, label, ok, detail})
	}

	switch {
	case sub == nil:
		add("subscription", "订阅有效", false, "找不到订阅")
	case sub.Status != service.SubscriptionStatusActive || !sub.ExpiresAt.After(now):
		add("subscription", "订阅有效", false, "订阅状态 "+sub.Status+"，到期 "+sub.ExpiresAt.Format("2006-01-02 15:04"))
	default:
		add("subscription", "订阅有效", true, "到期 "+sub.ExpiresAt.Format("2006-01-02 15:04"))
	}

	switch {
	case group == nil:
		add("seat_group", "订阅属于独享席位分组", false, "找不到分组")
	case !isProNativeSeat(group):
		add("seat_group", "订阅属于独享席位分组", false, "分组 "+group.Name+" 不是席位（席位名称应为 "+proNativeSeatPrefix+"N）")
	case group.Status != service.StatusActive || group.SubscriptionType != service.SubscriptionTypeSubscription:
		add("seat_group", "订阅属于独享席位分组", false, "分组 "+group.Name+" 未启用或不是订阅类型")
	default:
		add("seat_group", "订阅属于独享席位分组", true, group.Name)
	}

	bound := 0
	var usable *service.Account
	for i := range accounts {
		bound++
		if accounts[i].Platform == service.PlatformAnthropic && accounts[i].IsSchedulable() && usable == nil {
			usable = &accounts[i]
		}
	}
	switch {
	case bound != 1:
		add("account", "席位只绑定一个可用账号", false, fmt.Sprintf("席位绑定了 %d 个账号，应为 1 个", bound))
	case usable == nil:
		add("account", "席位只绑定一个可用账号", false, "绑定的账号不可调度（停用、限流或授权异常）")
	default:
		add("account", "席位只绑定一个可用账号", true, "账号 "+usable.Name)
	}

	onSeat, other := 0, []string{}
	for _, k := range keys {
		if k.Status != service.StatusActive {
			continue
		}
		if group != nil && k.GroupID == group.ID {
			onSeat++
		} else {
			other = append(other, "#"+strconv.FormatInt(k.ID, 10))
		}
	}
	switch {
	case onSeat == 0:
		detail := "用户没有挂在该席位上的启用 Key，请在用户管理里把 Key 的分组改为该席位或让用户重新创建"
		if len(other) > 0 {
			detail += "；这些启用 Key 在其他分组：" + strings.Join(other, "、")
		}
		add("key", "客户 Key 已挂在席位上", false, detail)
	case len(other) > 0:
		add("key", "客户 Key 已挂在席位上", true, fmt.Sprintf("%d 个 Key 在席位上；另有 Key %s 在其他分组，不影响本席位", onSeat, strings.Join(other, "、")))
	default:
		add("key", "客户 Key 已挂在席位上", true, fmt.Sprintf("%d 个 Key", onSeat))
	}
	return checks
}

func allChecksOK(checks []seatCheck) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return true
}

var seatVerifyMu sync.Mutex // one real upstream call at a time

// Verify handles POST /api/v1/admin/subscriptions/:id/verify-seat
func (h *SeatVerifyHandler) Verify(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	var req struct {
		Model string `json:"model"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Model == "" {
		req.Model = seatVerifyModels[0]
	}
	allowed := false
	for _, m := range seatVerifyModels {
		allowed = allowed || m == req.Model
	}
	if !allowed {
		response.BadRequest(c, "不支持的测试模型")
		return
	}
	if !seatVerifyMu.TryLock() {
		response.BadRequest(c, "已有一次测试正在进行，请稍后再试")
		return
	}
	defer seatVerifyMu.Unlock()

	ctx := c.Request.Context()
	sub, err := h.subscriptions.GetByID(ctx, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	group, err := h.users.GetGroup(ctx, sub.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !isProNativeSeat(group) {
		response.BadRequest(c, "只有 Claude 独享 Pro 席位的订阅可以测试")
		return
	}
	accounts, _, err := h.users.ListAccounts(ctx, 1, 50, "", "", "", "", group.ID, "", "id", "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	keys, _, err := h.users.GetUserAPIKeys(ctx, sub.UserID, 1, 100, "created_at", "desc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	views := make([]seatKeyView, 0, len(keys))
	var testKey string
	for i := range keys {
		gid := int64(0)
		if keys[i].GroupID != nil {
			gid = *keys[i].GroupID
		}
		views = append(views, seatKeyView{ID: keys[i].ID, Status: keys[i].Status, GroupID: gid})
		if testKey == "" && keys[i].Status == service.StatusActive && gid == group.ID {
			testKey = keys[i].Key
		}
	}
	checks := evaluateSeatStructure(sub, group, accounts, views, time.Now())

	callOK := false
	if allChecksOK(checks) && testKey != "" {
		checks = append(checks, h.gatewayCall(ctx, c, testKey, req.Model))
		callOK = checks[len(checks)-1].OK
	} else {
		checks = append(checks, seatCheck{"gateway", "用客户 Key 调用网关", false, "前面的检查未通过，未发起调用"})
	}

	var quota gin.H
	if rows, ok := h.usage.ProNativeSeatQuotaRows(ctx, group.ID); ok {
		quota = gin.H{}
		for _, r := range rows {
			meta, _ := r["extra"].(string)
			var m struct {
				Window   string  `json:"window"`
				ResetsAt *string `json:"resetsAt"`
			}
			_ = json.Unmarshal([]byte(meta), &m)
			quota[m.Window] = gin.H{"used": r["used"], "resets_at": m.ResetsAt}
		}
		checks = append(checks, seatCheck{"quota", "读取 5 小时 / 每周额度", true, "读取成功"})
	} else {
		checks = append(checks, seatCheck{"quota", "读取 5 小时 / 每周额度", false, "绑定账号无法返回额度（授权失效或上游暂不可用）"})
	}

	response.Success(c, gin.H{
		"ok":          allChecksOK(checks) && callOK,
		"model":       req.Model,
		"group":       group.Name,
		"checks":      checks,
		"quota":       quota,
		"verified_at": time.Now().Format(time.RFC3339),
	})
}

func (h *SeatVerifyHandler) gatewayCall(ctx context.Context, c *gin.Context, key, model string) seatCheck {
	const label = "用客户 Key 调用网关"
	port := "8080"
	if addr, ok := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		if _, p, err := net.SplitHostPort(addr.String()); err == nil {
			port = p
		}
	}
	body, _ := json.Marshal(gin.H{
		"model": model, "max_tokens": 8,
		"messages": []gin.H{{"role": "user", "content": "Reply with: ok"}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:"+port+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return seatCheck{"gateway", label, false, "无法构造请求"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	started := time.Now()
	resp, err := h.client.Do(req)
	if err != nil {
		return seatCheck{"gateway", label, false, "请求失败：" + err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return seatCheck{"gateway", label, false, fmt.Sprintf("HTTP %d：%s", resp.StatusCode, msg)}
	}
	var out struct {
		Usage struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(raw, &out)
	return seatCheck{"gateway", label, true, fmt.Sprintf("%s 调用成功，用时 %d ms，输入 %d / 输出 %d tokens", model, time.Since(started).Milliseconds(), out.Usage.Input, out.Usage.Output)}
}

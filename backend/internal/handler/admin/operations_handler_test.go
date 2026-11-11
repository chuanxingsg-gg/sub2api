package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationsUsersStub struct {
	service.AdminService
	target   *service.User
	created  *service.CreateUserInput
	updated  bool
	listRole string
}

func (s *operationsUsersStub) GetUser(context.Context, int64) (*service.User, error) {
	return s.target, nil
}
func (s *operationsUsersStub) UpdateUser(context.Context, int64, *service.UpdateUserInput) (*service.User, error) {
	s.updated = true
	return s.target, nil
}
func (s *operationsUsersStub) GetAllGroups(context.Context) ([]service.Group, error) {
	return []service.Group{{ID: 11, Name: "Claude-PAYG", Platform: service.PlatformAnthropic, Status: service.StatusActive}, {ID: 1, Name: "AWS-Claude", Status: service.StatusActive}}, nil
}
func (s *operationsUsersStub) CreateUser(_ context.Context, in *service.CreateUserInput) (*service.User, error) {
	s.created = in
	return &service.User{ID: 9, Role: in.Role}, nil
}
func (s *operationsUsersStub) ListUsers(_ context.Context, _, _ int, f service.UserListFilters, _, _ string) ([]service.User, int64, error) {
	s.listRole = f.Role
	return []service.User{{ID: 9, Role: service.RoleUser}, {ID: 1, Role: service.RoleAdmin}}, 1, nil
}

func operationsTestRequest(t *testing.T, role, method, path, body string, h *OperationsHandler, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
	}, OperationsRoleRequired)
	r.Handle(method, path, handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, strings.ReplaceAll(path, ":id", "1"), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestOperationsRejectsNonStaff(t *testing.T) {
	for _, role := range []string{"", service.RoleUser, "superadmin", "Operator"} {
		t.Run(role, func(t *testing.T) {
			w := operationsTestRequest(t, role, "GET", "/customers", "", nil, func(c *gin.Context) { t.Fatal("non-staff reached handler") })
			require.Equal(t, 403, w.Code)
		})
	}
}
func TestOperationsFiltersCustomersAndRedactsCredentials(t *testing.T) {
	s := &operationsUsersStub{}
	h := &OperationsHandler{users: s}
	w := operationsTestRequest(t, service.RoleOperator, "GET", "/customers", "", h, h.Customers)
	require.Equal(t, 200, w.Code)
	require.Equal(t, service.RoleUser, s.listRole)
	require.NotContains(t, w.Body.String(), `"role":"admin"`)
	require.NotContains(t, w.Body.String(), "password")
}
func TestOperationsCannotModifyStaff(t *testing.T) {
	for _, role := range []string{service.RoleAdmin, service.RoleOperator} {
		s := &operationsUsersStub{target: &service.User{ID: 1, Role: role}}
		h := &OperationsHandler{users: s}
		w := operationsTestRequest(t, service.RoleOperator, "PUT", "/customers/:id", `{"status":"disabled"}`, h, h.Update)
		require.Equal(t, 403, w.Code)
		require.False(t, s.updated)
	}
}
func TestOperationsRejectsPrivilegeInjection(t *testing.T) {
	for _, body := range []string{`{"role":"admin"}`, `{"role":"operator"}`, `{"balance":100}`, `{"allowed_groups":[1]}`, `{"Password":"takeover"}`, `{"password":"takeover"}`, `{"username":"a","username":"b"}`, `{"username":null}`, `{} {"role":"admin"}`, `{"concurrency":0}`, `{"concurrency":100}`} {
		t.Run(body, func(t *testing.T) {
			s := &operationsUsersStub{target: &service.User{ID: 1, Role: service.RoleUser}}
			h := &OperationsHandler{users: s}
			w := operationsTestRequest(t, service.RoleOperator, "PUT", "/customers/:id", body, h, h.Update)
			require.Equal(t, 400, w.Code)
			require.False(t, s.updated)
		})
	}
}
func TestOperationsCreatesOnlyRestrictedZeroBalanceCustomer(t *testing.T) {
	s := &operationsUsersStub{}
	h := &OperationsHandler{users: s}
	w := operationsTestRequest(t, service.RoleOperator, "POST", "/customers", `{"email":"customer@example.com","password":"valid-password-29"}`, h, h.Create)
	require.Equal(t, 200, w.Code)
	require.Equal(t, service.RoleUser, s.created.Role)
	require.Equal(t, 0.0, *s.created.Balance)
	require.Equal(t, 1, s.created.Concurrency)
	require.True(t, s.created.RestrictPublicGroups)
	require.Equal(t, []int64{11}, s.created.AllowedGroups)
}
func TestOperationsRechargeRequiresNotesAndIdempotency(t *testing.T) {
	for _, body := range []string{`{"amount":100}`, `{"amount":-100,"notes":"refund"}`, `{"amount":100,"notes":"received","operation":"set"}`, `{"amount":100,"notes":"received"}`} {
		s := &operationsUsersStub{target: &service.User{ID: 1, Role: service.RoleUser}}
		h := &OperationsHandler{users: s}
		w := operationsTestRequest(t, service.RoleOperator, "POST", "/customers/:id", body, h, h.Recharge)
		require.Equal(t, 400, w.Code)
	}
}

func TestOperationsCatalogScopesApprovedProducts(t *testing.T) {
	for _, test := range []struct {
		name, platform string
		allowed        bool
	}{
		{"GPT-Monthly-Basic", service.PlatformOpenAI, true},
		{"GPT-Monthly-Basic", service.PlatformAnthropic, false},
		{"Claude-Monthly-Basic", service.PlatformAnthropic, true},
		{"AWS-Claude", service.PlatformAnthropic, false},
		{"GPT-private", service.PlatformOpenAI, false},
		{"Grok-Monthly-Basic", service.PlatformGrok, true},
		{"Gemini-Monthly-Standard", service.PlatformGemini, true},
		{"Grok-Monthly-Basic", service.PlatformOpenAI, false},
		{"Gemini-Monthly-Advanced", service.PlatformAnthropic, false},
		{"Gemini-private", service.PlatformGemini, false},
	} {
		require.Equal(t, test.allowed, operationsGroup(&service.Group{Name: test.name, Platform: test.platform, Status: service.StatusActive}), test.name)
	}
	require.False(t, operationsGroup(&service.Group{Name: "GPT-Monthly-Basic", Platform: service.PlatformOpenAI, Status: service.StatusDisabled}))
}

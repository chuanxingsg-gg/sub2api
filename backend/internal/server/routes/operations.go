package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterOperationsRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwt middleware.JWTAuthMiddleware, audit middleware.AuditLogMiddleware, limiter *middleware.PanelRateLimiter) {
	ops := v1.Group("/operations")
	ops.Use(gin.HandlerFunc(jwt), admin.OperationsRoleRequired, limiter.Global(), gin.HandlerFunc(audit))
	handler := admin.NewOperationsHandler(h.Admin.User, h.Admin.Subscription, h.Admin.Payment)
	ops.GET("/customers", handler.Customers)
	ops.POST("/customers", handler.Create)
	ops.PUT("/customers/:id", handler.Update)
	ops.POST("/customers/:id/recharge", handler.Recharge)
	ops.GET("/customers/:id/subscriptions", handler.Subscriptions)
	ops.POST("/customers/:id/subscriptions", handler.Assign)
	ops.POST("/customers/:id/subscriptions/:subscription_id/extend", handler.Extend)
	ops.GET("/groups", handler.Groups)
	ops.GET("/plans", handler.Plans)
	ops.POST("/plans", handler.CreateCatalogPlan)
	ops.PUT("/plans/:plan_id", handler.UpdateCatalogPlan)
	v1.GET("/catalog/plans", handler.PublicCatalog)
}

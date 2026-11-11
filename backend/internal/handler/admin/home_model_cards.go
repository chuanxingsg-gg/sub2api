package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetHomeModelCards(c *gin.Context) {
	v, e := h.settingService.GetHomeModelCards(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, v)
}
func (h *SettingHandler) SaveHomeModelCards(c *gin.Context) {
	var v service.HomeModelCards
	if e := c.ShouldBindJSON(&v); e != nil {
		response.BadRequest(c, "卡片配置格式无效")
		return
	}
	if e := service.ValidateHomeModelCards(v); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	if e := h.settingService.SaveHomeModelCards(c.Request.Context(), v); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}

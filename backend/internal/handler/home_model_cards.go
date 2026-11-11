package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
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

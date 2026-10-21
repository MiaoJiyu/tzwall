package handler

import (
	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// InteractionHandler 点赞/点踩接口。
type InteractionHandler struct {
	svc *service.InteractionService
}

func NewInteractionHandler(svc *service.InteractionService) *InteractionHandler {
	return &InteractionHandler{svc: svc}
}

// React 表态。attitude: 1 赞 / -1 踩 / 0 取消。
func (h *InteractionHandler) React(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var req struct {
		TargetType string `json:"target_type"`
		TargetID   uint   `json:"target_id"`
		Attitude   int    `json:"attitude"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.React(u, req.TargetType, req.TargetID, req.Attitude); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

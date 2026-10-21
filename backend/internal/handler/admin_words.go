package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// WordHandler 词表管理（仅超级管理员）。
type WordHandler struct {
	svc *service.WordFilterService
}

func NewWordHandler(svc *service.WordFilterService) *WordHandler {
	return &WordHandler{svc: svc}
}

// List 词条列表。
func (h *WordHandler) List(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	if pageSize > 200 {
		pageSize = 200
	}
	items, total, err := h.svc.List(c.Query("category"), c.Query("keyword"), page, pageSize)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// Add 新增词条（支持批量：换行/逗号分隔）。
func (h *WordHandler) Add(c *gin.Context) {
	u := requireSuperAdmin(c)
	if u == nil {
		return
	}
	var req struct {
		Words    string `json:"words"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	list := splitWords(req.Words)
	if len(list) == 0 {
		response.FailCode(c, errs.ErrValidation, "请填写词条")
		return
	}
	added, err := h.svc.Add(u, list, req.Category)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"added": added})
}

// Remove 删除词条。
func (h *WordHandler) Remove(c *gin.Context) {
	u := requireSuperAdmin(c)
	if u == nil {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Remove(u, uint(id)); err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, nil)
}

// splitWords 支持换行、逗号、分号分隔的批量导入。
func splitWords(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == '，' || r == ';' || r == '；'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

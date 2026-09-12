package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ManagerHandler handles department-manager scoped endpoints.
// All handlers resolve the manager ID from the JWT subject and re-validate
// department scope on the backend; IDs in the URL are never trusted alone.
type ManagerHandler struct {
	managerService *service.ManagerService
}

func NewManagerHandler(managerService *service.ManagerService) *ManagerHandler {
	return &ManagerHandler{managerService: managerService}
}

// ManagerMemberDTO 是经理视图下的成员条目（不含余额/备注等管理侧敏感字段）。
type ManagerMemberDTO struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Status   string `json:"status"`

	PrimaryDeptID *int64 `json:"primary_dept_id,omitempty"`

	Subscriptions []dto.UserSubscription `json:"subscriptions"`
}

// Members lists department members (with existing subscription records) in the
// manager's scope. GET /api/v1/manager/members
func (h *ManagerHandler) Members(c *gin.Context) {
	managerID := managerIDFromContext(c)
	page, pageSize := response.ParsePagination(c)

	userIDs, pag, err := h.managerService.ListScopedMembers(c.Request.Context(), managerID, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]ManagerMemberDTO, 0, len(userIDs))
	for _, userID := range userIDs {
		user, err := h.managerService.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		subs, err := h.managerService.ListMemberSubscriptions(c.Request.Context(), managerID, userID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		out = append(out, managerMemberToDTO(user, subs))
	}
	response.PaginatedWithResult(c, out, toManagerResponsePagination(pag))
}

// SubscriptionProgress returns usage progress for one member subscription.
// GET /api/v1/manager/subscriptions/:id/progress
func (h *ManagerHandler) SubscriptionProgress(c *gin.Context) {
	managerID := managerIDFromContext(c)
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	progress, err := h.managerService.GetSubscriptionProgressInScope(c.Request.Context(), managerID, subscriptionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, progress)
}

// ResetQuotaRequest mirrors the admin reset-quota payload.
type ResetQuotaRequest struct {
	Daily   bool `json:"daily"`
	Weekly  bool `json:"weekly"`
	Monthly bool `json:"monthly"`
}

// ResetQuota resets daily/weekly/monthly usage windows for one member
// subscription after scope validation. POST /api/v1/manager/subscriptions/:id/reset-quota
func (h *ManagerHandler) ResetQuota(c *gin.Context) {
	managerID := managerIDFromContext(c)
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	var req ResetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if !req.Daily && !req.Weekly && !req.Monthly {
		response.BadRequest(c, "At least one of 'daily', 'weekly', or 'monthly' must be true")
		return
	}

	sub, err := h.managerService.ResetMemberQuota(c.Request.Context(), managerID, subscriptionID, req.Daily, req.Weekly, req.Monthly)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UserSubscriptionFromService(sub))
}

func managerMemberToDTO(user *service.User, subs []service.UserSubscription) ManagerMemberDTO {
	out := ManagerMemberDTO{
		ID:            user.ID,
		Email:         user.Email,
		Username:      user.Username,
		Role:          user.Role,
		Status:        user.Status,
		PrimaryDeptID: user.PrimaryDeptID,
		Subscriptions: make([]dto.UserSubscription, 0, len(subs)),
	}
	for i := range subs {
		out.Subscriptions = append(out.Subscriptions, *dto.UserSubscriptionFromService(&subs[i]))
	}
	return out
}

func toManagerResponsePagination(p *pagination.PaginationResult) *response.PaginationResult {
	if p == nil {
		return nil
	}
	return &response.PaginationResult{
		Total:    p.Total,
		Page:     p.Page,
		PageSize: p.PageSize,
		Pages:    p.Pages,
	}
}

func managerIDFromContext(c *gin.Context) int64 {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}

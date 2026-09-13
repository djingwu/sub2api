package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ManagerScopeHandler exposes the administrator-only manager department scope
// configuration. It deliberately keeps manager selection separate from the
// general user list so the role filter cannot be omitted by the client.
type ManagerScopeHandler struct {
	adminService   service.AdminService
	managerService *service.ManagerService
}

func NewManagerScopeHandler(adminService service.AdminService, managerService *service.ManagerService) *ManagerScopeHandler {
	return &ManagerScopeHandler{adminService: adminService, managerService: managerService}
}

type ManagerScopeDepartmentDTO struct {
	DeptID   int64  `json:"dept_id"`
	ParentID int64  `json:"parent_id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type ManagerScopeManagerDTO struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	PrimaryDeptID *int64 `json:"primary_dept_id,omitempty"`
}

type ReplaceManagerDepartmentsRequest struct {
	DepartmentIDs []int64 `json:"department_ids"`
}

func (h *ManagerScopeHandler) ListDepartments(c *gin.Context) {
	departments, err := h.managerService.ListDepartments(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]ManagerScopeDepartmentDTO, 0, len(departments))
	for _, department := range departments {
		out = append(out, managerScopeDepartmentToDTO(department))
	}
	response.Success(c, out)
}

func (h *ManagerScopeHandler) ListManagers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	includeSubscriptions := false
	users, total, err := h.adminService.ListUsers(
		c.Request.Context(),
		page,
		pageSize,
		service.UserListFilters{Role: service.RoleManager, IncludeSubscriptions: &includeSubscriptions},
		"created_at",
		"desc",
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]ManagerScopeManagerDTO, 0, len(users))
	for _, user := range users {
		out = append(out, managerScopeManagerToDTO(&user))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *ManagerScopeHandler) ListManagerDepartments(c *gin.Context) {
	managerID, ok := parseManagerID(c)
	if !ok {
		return
	}
	if !h.ensureManager(c, managerID) {
		return
	}
	departments, err := h.managerService.ListManagerDepartments(c.Request.Context(), managerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]ManagerScopeDepartmentDTO, 0, len(departments))
	for _, department := range departments {
		out = append(out, managerScopeDepartmentToDTO(department))
	}
	response.Success(c, out)
}

func (h *ManagerScopeHandler) ReplaceManagerDepartments(c *gin.Context) {
	managerID, ok := parseManagerID(c)
	if !ok {
		return
	}
	if !h.ensureManager(c, managerID) {
		return
	}
	var req ReplaceManagerDepartmentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	for _, departmentID := range req.DepartmentIDs {
		if departmentID <= 0 {
			response.BadRequest(c, "department_ids must contain only positive IDs")
			return
		}
	}
	if err := h.managerService.ReplaceManagerDepartments(c.Request.Context(), managerID, req.DepartmentIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	departments, err := h.managerService.ListManagerDepartments(c.Request.Context(), managerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]ManagerScopeDepartmentDTO, 0, len(departments))
	for _, department := range departments {
		out = append(out, managerScopeDepartmentToDTO(department))
	}
	response.Success(c, out)
}

func (h *ManagerScopeHandler) ensureManager(c *gin.Context, managerID int64) bool {
	user, err := h.adminService.GetUser(c.Request.Context(), managerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	if user.Role != service.RoleManager {
		response.BadRequest(c, "user is not a department manager")
		return false
	}
	return true
}

func parseManagerID(c *gin.Context) (int64, bool) {
	managerID, err := strconv.ParseInt(c.Param("manager_id"), 10, 64)
	if err != nil || managerID <= 0 {
		response.BadRequest(c, "Invalid manager ID")
		return 0, false
	}
	return managerID, true
}

func managerScopeDepartmentToDTO(department service.DingTalkDepartment) ManagerScopeDepartmentDTO {
	return ManagerScopeDepartmentDTO{
		DeptID:   department.DeptID,
		ParentID: department.ParentID,
		Name:     department.Name,
		IsActive: department.IsActive,
	}
}

func managerScopeManagerToDTO(user *service.User) ManagerScopeManagerDTO {
	return ManagerScopeManagerDTO{
		ID:            user.ID,
		Email:         user.Email,
		Username:      user.Username,
		Role:          user.Role,
		Status:        user.Status,
		PrimaryDeptID: user.PrimaryDeptID,
	}
}

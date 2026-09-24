package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SaturationHandler serves the admin user saturation report.
type SaturationHandler struct {
	saturationService *service.SaturationService
}

// NewSaturationHandler creates a saturation report handler.
func NewSaturationHandler(saturationService *service.SaturationService) *SaturationHandler {
	return &SaturationHandler{saturationService: saturationService}
}

// Snapshot returns the saturation snapshot for every quota holder.
// GET /api/v1/admin/saturation
func (h *SaturationHandler) Snapshot(c *gin.Context) {
	snapshot, err := h.saturationService.GetSnapshot(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}

// Users returns the paginated saturation user list for every quota holder.
// GET /api/v1/admin/saturation/users
func (h *SaturationHandler) Users(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	list, err := h.saturationService.ListUsers(c.Request.Context(), service.SaturationUserQuery{
		Metric:     c.Query("metric"),
		Band:       c.Query("band"),
		Compliance: c.Query("compliance"),
		Search:     c.Query("search"),
		Sort:       c.Query("sort"),
		Order:      c.Query("order"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}

// Trend returns the daily subscription-billed usage trend.
// GET /api/v1/admin/saturation/trend?days=30
func (h *SaturationHandler) Trend(c *gin.Context) {
	days := 0
	if raw := c.Query("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			response.BadRequest(c, "Invalid days parameter")
			return
		}
		days = parsed
	}
	points, err := h.saturationService.GetDailyTrend(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, points)
}

// GetConfig returns the baseline-combo configuration.
// GET /api/v1/admin/saturation/config
func (h *SaturationHandler) GetConfig(c *gin.Context) {
	response.Success(c, h.saturationService.GetConfig(c.Request.Context()))
}

// UpdateConfig replaces the baseline-combo configuration.
// PUT /api/v1/admin/saturation/config
func (h *SaturationHandler) UpdateConfig(c *gin.Context) {
	var req usagestats.SaturationConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	config, err := h.saturationService.UpdateConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

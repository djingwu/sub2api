package handler

import (
	"context"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPlazaHandler 处理「模型广场」查询。
//
// 广场路由挂 OptionalJWT 中间件：匿名可访问（除非 require_auth 开启），带 token 则
// 识别用户。所有分组（含专属分组）对访客一律展示，无需授权；模型列表可按
// model_plaza_models 白名单裁剪。
type ModelPlazaHandler struct {
	channelService modelPlazaChannelService
	apiKeyService  modelPlazaAPIKeyService
	settingService modelPlazaSettingService
}

// modelPlazaChannelService 广场渠道数据源（由 *service.ChannelService 实现）。
type modelPlazaChannelService interface {
	ListPlazaGroups(ctx context.Context, modelWhitelist []string) ([]service.PlazaGroup, error)
}

// modelPlazaSettingService 广场运行时设置源（由 *service.SettingService 实现）。
type modelPlazaSettingService interface {
	GetModelPlazaRuntime(ctx context.Context) service.ModelPlazaRuntime
}

// modelPlazaAPIKeyService 用户专属倍率源（由 *service.APIKeyService 实现）。
type modelPlazaAPIKeyService interface {
	GetUserGroupRates(ctx context.Context, userID int64) (map[int64]float64, error)
}

// NewModelPlazaHandler 创建模型广场 handler。
func NewModelPlazaHandler(
	channelService *service.ChannelService,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
) *ModelPlazaHandler {
	return &ModelPlazaHandler{
		channelService: channelService,
		apiKeyService:  apiKeyService,
		settingService: settingService,
	}
}

// modelPlazaOfficialPricing LiteLLM 官方参考价（USD per token）。
type modelPlazaOfficialPricing struct {
	InputPrice        *float64 `json:"input_price"`
	OutputPrice       *float64 `json:"output_price"`
	CacheWritePrice   *float64 `json:"cache_write_price"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price,omitempty"`
	CacheReadPrice    *float64 `json:"cache_read_price"`
}

// modelPlazaModel 广场模型条目：渠道定价（白名单形态）+ 官方参考价。
type modelPlazaModel struct {
	Name            string                     `json:"name"`
	Platform        string                     `json:"platform"`
	Pricing         *userSupportedModelPricing `json:"pricing"`
	OfficialPricing *modelPlazaOfficialPricing `json:"official_pricing"`
}

// modelPlazaGroup 广场分组条目（白名单字段）。
type modelPlazaGroup struct {
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Platform           string   `json:"platform"`
	SubscriptionType   string   `json:"subscription_type"`
	RateMultiplier     float64  `json:"rate_multiplier"`
	UserRateMultiplier *float64 `json:"user_rate_multiplier,omitempty"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakStart          string   `json:"peak_start"`
	PeakEnd            string   `json:"peak_end"`
	PeakRateMultiplier float64  `json:"peak_rate_multiplier"`
	IsExclusive        bool     `json:"is_exclusive"`
	// 生图独立倍率：为 true 时图片计费模型的实付倍率取 ImageRateMultiplier，
	// 不取分组/用户专属倍率。
	ImageRateIndependent bool              `json:"image_rate_independent"`
	ImageRateMultiplier  float64           `json:"image_rate_multiplier"`
	Models               []modelPlazaModel `json:"models"`
}

// modelPlazaResponse 广场页响应。
type modelPlazaResponse struct {
	Description string            `json:"description"`
	Groups      []modelPlazaGroup `json:"groups"`
}

// Get 返回模型广场数据。
// GET /api/v1/model-plaza
func (h *ModelPlazaHandler) Get(c *gin.Context) {
	if h.settingService == nil {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}
	rt := h.settingService.GetModelPlazaRuntime(c.Request.Context())
	if !rt.Enabled {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}

	subject, authed := middleware.GetAuthSubjectFromContext(c)
	if rt.RequireAuth && !authed {
		response.Unauthorized(c, "Authentication required")
		return
	}

	groups, err := h.channelService.ListPlazaGroups(c.Request.Context(), rt.Models)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 用户专属倍率仅是展示增强，失败降级为分组默认倍率。
	var userRates map[int64]float64
	if authed {
		userRates, err = h.apiKeyService.GetUserGroupRates(c.Request.Context(), subject.UserID)
		if err != nil {
			slog.Warn("model_plaza_user_rates_failed", "error", err, "user_id", subject.UserID)
			userRates = nil
		}
	}

	out := make([]modelPlazaGroup, 0, len(groups))
	for i := range groups {
		out = append(out, toModelPlazaGroupDTO(&groups[i], userRates))
	}
	response.Success(c, modelPlazaResponse{
		Description: rt.Description,
		Groups:      out,
	})
}

// toModelPlazaGroupDTO 将 service 层广场分组映射为白名单 DTO,并合并用户专属倍率。
func toModelPlazaGroupDTO(g *service.PlazaGroup, userRates map[int64]float64) modelPlazaGroup {
	models := make([]modelPlazaModel, 0, len(g.Models))
	for i := range g.Models {
		m := &g.Models[i]
		models = append(models, modelPlazaModel{
			Name:            m.Name,
			Platform:        m.Platform,
			Pricing:         toUserPricing(m.Pricing),
			OfficialPricing: toModelPlazaOfficialPricing(m.OfficialPricing),
		})
	}
	dto := modelPlazaGroup{
		ID:                   g.ID,
		Name:                 g.Name,
		Description:          g.Description,
		Platform:             g.Platform,
		SubscriptionType:     g.SubscriptionType,
		RateMultiplier:       g.RateMultiplier,
		PeakRateEnabled:      g.PeakRateEnabled,
		PeakStart:            g.PeakStart,
		PeakEnd:              g.PeakEnd,
		PeakRateMultiplier:   g.PeakRateMultiplier,
		IsExclusive:          g.IsExclusive,
		ImageRateIndependent: g.ImageRateIndependent,
		ImageRateMultiplier:  g.ImageRateMultiplier,
		Models:               models,
	}
	if rate, ok := userRates[g.ID]; ok {
		dto.UserRateMultiplier = &rate
	}
	return dto
}

// toModelPlazaOfficialPricing 转换官方参考价；nil 透传（前端显示 "-"）。
func toModelPlazaOfficialPricing(p *service.PlazaOfficialPricing) *modelPlazaOfficialPricing {
	if p == nil {
		return nil
	}
	return &modelPlazaOfficialPricing{
		InputPrice:        p.InputPrice,
		OutputPrice:       p.OutputPrice,
		CacheWritePrice:   p.CacheWritePrice,
		CacheWrite1hPrice: p.CacheWrite1hPrice,
		CacheReadPrice:    p.CacheReadPrice,
	}
}

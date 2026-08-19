package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/ent/group"
)

// BindUserToDingTalkDeptGroup 将钉钉用户按部门匹配绑定到专属订阅分组：
//
//  1. 读取设置项 dingtalk_dept_group_map（dept_id → 部门名）；
//  2. 用户直属部门 ID 未命中映射 → 直接返回 nil（不分配任何分组/订阅）；
//  3. 命中 → 找到以部门名命名的专属订阅分组（预建，迁移 226），
//     幂等绑定订阅（AssignSubscription：已存在且未过期保持原样，不续期不累加；
//     过期后重激活），有效期取分组 default_validity_days（30 天）。
//
// 分组账号池需管理员把该部门使用的上游 gpt 账号挂到对应分组。
// 订阅额度走分组配置（monthly_limit_usd，迁移 227 预设 $400）。
// 任何失败均返回错误，调用方（钉钉登录同步）只记日志不阻断登录。
func (s *AuthService) BindUserToDingTalkDeptGroup(ctx context.Context, userID, deptID int64) error {
	if s == nil || s.entClient == nil || s.defaultSubAssigner == nil || s.settingService == nil || userID <= 0 || deptID <= 1 {
		return nil
	}

	mapping, err := s.settingService.GetDingTalkDeptGroupMap(ctx)
	if err != nil {
		return fmt.Errorf("load dingtalk dept group map: %w", err)
	}
	deptName, ok := mapping[deptID]
	if !ok || strings.TrimSpace(deptName) == "" {
		// 未匹配：不分配分组
		return nil
	}
	deptName = strings.TrimSpace(deptName)

	g, err := s.entClient.Group.Query().
		Where(group.NameEQ(deptName), group.DeletedAtIsNil()).
		First(ctx)
	if err != nil {
		return fmt.Errorf("query dept group %q: %w", deptName, err)
	}

	validityDays := g.DefaultValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}

	if _, err := s.defaultSubAssigner.AssignSubscription(ctx, &AssignSubscriptionInput{
		UserID:       userID,
		GroupID:      g.ID,
		ValidityDays: validityDays,
		AssignedBy:   userID,
		Notes:        "auto bound to dept group on dingtalk login",
	}); err != nil {
		return fmt.Errorf("assign subscription for dept group %d: %w", g.ID, err)
	}
	return nil
}
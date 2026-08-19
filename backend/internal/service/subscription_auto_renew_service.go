package service

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
)

const (
	// subscriptionAutoRenewLeaderLockKey gates the periodic auto-renew scan so
	// that only one instance renews due subscriptions across multi-instance
	// deployments, avoiding double extensions.
	subscriptionAutoRenewLeaderLockKey = "subscription:auto-renew:leader"
	// subscriptionAutoRenewLeaderLockTTL bounds crash recovery; the scan can
	// page through many subscriptions, so keep it comfortably above one cycle.
	subscriptionAutoRenewLeaderLockTTL = 5 * time.Minute
)

const (
	// subscriptionAutoRenewScanPageSize 每页扫描的订阅数。
	subscriptionAutoRenewScanPageSize = 200
)

// AutoRenewIdentityRepository 自动续期所需的窄接口：查询用户绑定的认证身份。
// 生产实现为 UserRepository（已包含 ListUserAuthIdentities），窄接口便于测试桩。
type AutoRenewIdentityRepository interface {
	ListUserAuthIdentities(ctx context.Context, userID int64) ([]UserAuthIdentityRecord, error)
}

// SubscriptionAutoRenewService 定期为临期/已过期的订阅执行免费自动续期。
// 仅对钉钉用户（注册来源或绑定身份为 dingtalk）生效；关闭 auto_renew 的订阅跳过。
// 续期天数取分组在售套餐的最短有效期，无套餐时默认 30 天。
type SubscriptionAutoRenewService struct {
	userSubRepo     UserSubscriptionRepository
	identityRepo    AutoRenewIdentityRepository
	subscriptionSvc *SubscriptionService
	interval        time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

// NewSubscriptionAutoRenewService 创建自动续期服务。
func NewSubscriptionAutoRenewService(userSubRepo UserSubscriptionRepository, identityRepo AutoRenewIdentityRepository, subscriptionSvc *SubscriptionService, interval time.Duration) *SubscriptionAutoRenewService {
	return &SubscriptionAutoRenewService{
		userSubRepo:     userSubRepo,
		identityRepo:    identityRepo,
		subscriptionSvc: subscriptionSvc,
		interval:        interval,
		stopCh:          make(chan struct{}),
		instanceID:      uuid.NewString(),
	}
}

// SetLeaderLock 注入 leader lock 缓存与 DB，用于多实例下选举唯一扫描者。
func (s *SubscriptionAutoRenewService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *SubscriptionAutoRenewService) Start() {
	if s == nil || s.userSubRepo == nil || s.subscriptionSvc == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *SubscriptionAutoRenewService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *SubscriptionAutoRenewService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	renewedTotal, err := s.renewDueSubscriptions(ctx)
	if err != nil {
		log.Printf("[SubscriptionAutoRenew] Auto renew failed: %v", err)
		return
	}
	if renewedTotal > 0 {
		log.Printf("[SubscriptionAutoRenew] Auto renewed %d subscriptions", renewedTotal)
	}
}

// renewDueSubscriptions 扫描临近到期/已过期且开启自动续期的订阅并免费续期。
// 多实例下仅 leader 执行，防止重复续期。
func (s *SubscriptionAutoRenewService) renewDueSubscriptions(ctx context.Context) (int, error) {
	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, subscriptionAutoRenewLeaderLockKey, s.instanceID, subscriptionAutoRenewLeaderLockTTL)
	if !ok {
		return 0, nil
	}
	defer release()

	now := time.Now()
	deadline := now.Add(subscriptionAutoRenewLeadTime)
	renewedTotal := 0
	for page := 1; ; page++ {
		subs, pag, err := s.userSubRepo.ListDueAutoRenew(ctx, deadline, pagination.PaginationParams{Page: page, PageSize: subscriptionAutoRenewScanPageSize})
		if err != nil {
			return renewedTotal, err
		}
		for i := range subs {
			sub := &subs[i]
			if !s.isEligibleForAutoRenew(ctx, sub) {
				continue
			}
			if err := s.renewOne(ctx, sub); err != nil {
				log.Printf("[SubscriptionAutoRenew] Renew subscription %d failed: %v", sub.ID, err)
				continue
			}
			renewedTotal++
		}
		if pag == nil || page >= pag.Pages || len(subs) == 0 {
			return renewedTotal, nil
		}
	}
}

// isEligibleForAutoRenew 判断订阅是否满足免费自动续期条件（钉钉用户 + 未撤销 + 开启自动续期）。
func (s *SubscriptionAutoRenewService) isEligibleForAutoRenew(ctx context.Context, sub *UserSubscription) bool {
	if sub == nil || !sub.AutoRenew {
		return false
	}
	if s.subscriptionSvc.userIsDingTalk(sub.User) {
		return true
	}
	if s.identityRepo != nil {
		records, err := s.identityRepo.ListUserAuthIdentities(ctx, sub.UserID)
		if err != nil {
			log.Printf("[SubscriptionAutoRenew] List dingtalk identity for user %d failed: %v", sub.UserID, err)
			return false
		}
		for _, record := range records {
			if strings.EqualFold(strings.TrimSpace(record.ProviderType), "dingtalk") {
				return true
			}
		}
	}
	return false
}

func (s *SubscriptionAutoRenewService) renewOne(ctx context.Context, sub *UserSubscription) error {
	days := s.subscriptionSvc.AutoRenewValidityDays(ctx, sub.GroupID)
	_, err := s.subscriptionSvc.FreeRenewSubscription(ctx, sub.ID, days, subscriptionAutoRenewNote)
	return err
}
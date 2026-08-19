package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type handlerAutoRenewRepoStub struct {
	service.UserSubscriptionRepository
	sub *service.UserSubscription
}

// GetByID 覆盖 SetAutoRenew / 属主校验路径。
func (r *handlerAutoRenewRepoStub) GetByID(context.Context, int64) (*service.UserSubscription, error) {
	if r.sub == nil {
		return nil, service.ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

// UpdateAutoRenew 覆盖 SetAutoRenew 写入路径。
func (r *handlerAutoRenewRepoStub) UpdateAutoRenew(_ context.Context, _ int64, enabled bool) (*service.UserSubscription, error) {
	cp := *r.sub
	cp.AutoRenew = enabled
	r.sub = &cp
	return &cp, nil
}

// 其余接口方法不参与本测试（嵌入签名满足编译）。
type autoRenewCreated struct{}

func newAutoRenewRouter(t *testing.T, repo *handlerAutoRenewRepoStub) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewSubscriptionHandler(service.NewSubscriptionService(nil, repo, nil, nil, nil))
	router.PUT("/subscriptions/:id/auto-renew", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		h.UpdateAutoRenew(c)
	})
	return router
}

func TestUpdateAutoRenewSuccess(t *testing.T) {
	repo := &handlerAutoRenewRepoStub{sub: &service.UserSubscription{ID: 7, UserID: 42, GroupID: 3, AutoRenew: true}}
	router := newAutoRenewRouter(t, repo)

	req := httptest.NewRequest(http.MethodPut, "/subscriptions/7/auto-renew", strings.NewReader(`{"enabled":false}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, repo.sub.AutoRenew)

	var body struct {
		Data struct {
			ID        int64 `json:"id"`
			AutoRenew bool   `json:"auto_renew"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, int64(7), body.Data.ID)
	require.False(t, body.Data.AutoRenew)
}

func TestUpdateAutoRenewRejectsOthersSubscription(t *testing.T) {
	repo := &handlerAutoRenewRepoStub{sub: &service.UserSubscription{ID: 7, UserID: 999, GroupID: 3, AutoRenew: true}}
	router := newAutoRenewRouter(t, repo)

	req := httptest.NewRequest(http.MethodPut, "/subscriptions/7/auto-renew", strings.NewReader(`{"enabled":false}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.True(t, repo.sub.AutoRenew, "switch 不应被更改")
}

func TestUpdateAutoRenewRejectsInvalidInput(t *testing.T) {
	repo := &handlerAutoRenewRepoStub{sub: &service.UserSubscription{ID: 7, UserID: 42, GroupID: 3, AutoRenew: true}}
	router := newAutoRenewRouter(t, repo)

	for _, body := range []string{`{}`, `{"enabled":"yes"}`, ``, `not-json`} {
		req := httptest.NewRequest(http.MethodPut, "/subscriptions/7/auto-renew", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body=%q", body)
		require.True(t, repo.sub.AutoRenew, "body=%q", body)
	}
}

func TestUpdateAutoRenewNotFound(t *testing.T) {
	router := newAutoRenewRouter(t, &handlerAutoRenewRepoStub{})

	req := httptest.NewRequest(http.MethodPut, "/subscriptions/7/auto-renew", strings.NewReader(`{"enabled":true}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
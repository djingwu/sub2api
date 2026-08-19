package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

func TestFilterAccountsByAllowedUsers_EmptyWhitelistAllowsAll(t *testing.T) {
	svc := &GatewayService{}
	accounts := []Account{
		{ID: 1, AllowedUserIDs: nil},
		{ID: 2, AllowedUserIDs: []int64{}},
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(100))
	out := svc.filterAccountsByAllowedUsers(ctx, accounts)
	if len(out) != 2 {
		t.Fatalf("expected all accounts when no whitelist configured, got %d", len(out))
	}
}

func TestFilterAccountsByAllowedUsers_WhitelistRestrictsOthers(t *testing.T) {
	svc := &GatewayService{}
	accounts := []Account{
		{ID: 1, AllowedUserIDs: []int64{7, 9}},
		{ID: 2, AllowedUserIDs: []int64{}},
		{ID: 3, AllowedUserIDs: []int64{9}},
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(7))
	out := svc.filterAccountsByAllowedUsers(ctx, accounts)
	if len(out) != 2 {
		t.Fatalf("expected 2 accounts (ID=1 whitelisted, ID=2 unrestricted), got %d", len(out))
	}
	for _, acc := range out {
		if acc.ID == 3 {
			t.Fatal("account 3 must be filtered out for user 7")
		}
	}
}

func TestFilterAccountsByAllowedUsers_UnauthenticatedUnrestricted(t *testing.T) {
	svc := &GatewayService{}
	accounts := []Account{{ID: 1, AllowedUserIDs: []int64{7}}}
	out := svc.filterAccountsByAllowedUsers(context.Background(), accounts)
	if len(out) != 1 {
		t.Fatalf("expected account kept when request user cannot be resolved (backward compat), got %d", len(out))
	}
}

func TestIsAccountAllowedForUser(t *testing.T) {
	svc := &GatewayService{}
	whitelisted := &Account{ID: 1, AllowedUserIDs: []int64{7}}
	if !svc.isAccountAllowedForUser(context.Background(), whitelisted) {
		t.Fatal("unresolved user must not be blocked (backward compat)")
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(8))
	if svc.isAccountAllowedForUser(ctx, whitelisted) {
		t.Fatal("user 8 must be blocked by account whitelist of [7]")
	}
	ctxOK := context.WithValue(context.Background(), ctxkey.UserID, int64(7))
	if !svc.isAccountAllowedForUser(ctxOK, whitelisted) {
		t.Fatal("user 7 must be allowed by account whitelist of [7]")
	}
	if !svc.isAccountAllowedForUser(ctx, &Account{ID: 2}) {
		t.Fatal("account without whitelist must be allowed for any user")
	}
}
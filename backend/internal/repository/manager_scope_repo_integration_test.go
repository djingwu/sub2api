//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

// ManagerScopeRepoSuite 覆盖经理可见成员的两条口径并集：
//  1. users.primary_dept_id 命中管辖部门树（含子孙）；
//  2. 用户订阅分组名落在 dingtalk_dept_group_map 映射出的分组集合。
type ManagerScopeRepoSuite struct {
	suite.Suite
	ctx    context.Context
	client *dbent.Client
	tx     *dbent.Tx
	repo   *managerScopeRepository
}

func (s *ManagerScopeRepoSuite) SetupTest() {
	s.ctx = context.Background()
	s.tx = testEntTx(s.T())
	s.client = s.tx.Client()
	// 读路径只依赖 sqlExecutor；ent.Tx 实现 QueryContext。
	s.repo = &managerScopeRepository{sql: s.tx}
}

func TestManagerScopeRepoSuite(t *testing.T) {
	suite.Run(t, new(ManagerScopeRepoSuite))
}

// seedDeptGroupMap 写入本测试专用的部门→分组映射（事务内，自动回滚）。
func (s *ManagerScopeRepoSuite) seedDeptGroupMap(json string) {
	s.T().Helper()
	_, err := s.tx.ExecContext(s.ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES ('dingtalk_dept_group_map', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		json)
	s.Require().NoError(err, "seed dingtalk_dept_group_map")
}

func (s *ManagerScopeRepoSuite) mustUpsertDept(deptID, parentID int64, name string) {
	s.T().Helper()
	err := s.repo.UpsertDepartment(s.ctx, &service.DingTalkDepartment{
		DeptID:   deptID,
		ParentID: parentID,
		Name:     name,
		IsActive: true,
	})
	s.Require().NoError(err, "upsert department %d", deptID)
}

func (s *ManagerScopeRepoSuite) mustAssignManagerDept(managerUserID, deptID int64) {
	s.T().Helper()
	_, err := s.tx.ExecContext(s.ctx, `
		INSERT INTO user_manager_departments (manager_user_id, dept_id)
		VALUES ($1, $2)
		ON CONFLICT (manager_user_id, dept_id) DO NOTHING`,
		managerUserID, deptID)
	s.Require().NoError(err, "assign manager dept")
}

func (s *ManagerScopeRepoSuite) mustCreateUserWithEmail(email string) *service.User {
	s.T().Helper()
	return mustCreateUser(s.T(), s.client, &service.User{Email: email})
}

func (s *ManagerScopeRepoSuite) setPrimaryDept(userID, deptID int64) {
	s.T().Helper()
	_, err := s.client.User.UpdateOneID(userID).SetPrimaryDeptID(deptID).Save(s.ctx)
	s.Require().NoError(err, "set primary dept")
}

func (s *ManagerScopeRepoSuite) mustCreateGroupNamed(name string) *service.Group {
	s.T().Helper()
	return mustCreateGroup(s.T(), s.client, &service.Group{Name: name})
}

func (s *ManagerScopeRepoSuite) mustSubscribe(userID, groupID int64) {
	s.T().Helper()
	mustCreateSubscription(s.T(), s.client, &service.UserSubscription{
		UserID:  userID,
		GroupID: groupID,
	})
}

// TestScope_UnionOfPrimaryDeptAndSubscriptionGroup 验证两条口径取并集：
// 主部门命中管辖树的用户、以及仅靠订阅分组名命中的用户都可见；
// 两者都不命中的用户不可见。
func (s *ManagerScopeRepoSuite) TestScope_UnionOfPrimaryDeptAndSubscriptionGroup() {
	const (
		rootDept  = int64(910000001)
		childDept = int64(910000002)
		otherDept = int64(910000003)
	)

	// 分组名避开迁移种子数据里已有的名字，保证 groups_name_unique_active 不冲突。
	const groupBigName = "大数据部-union-test"

	s.seedDeptGroupMap(`{"910000001": "大数据部-union-test"}`)
	s.mustUpsertDept(rootDept, 0, "根-大数据部")
	s.mustUpsertDept(childDept, rootDept, "子-数仓组")
	s.mustUpsertDept(otherDept, 0, "无关部门")

	manager := s.mustCreateUserWithEmail("mgr-scope@test.com")
	_, err := s.client.User.UpdateOneID(manager.ID).SetRole(service.RoleManager).Save(s.ctx)
	s.Require().NoError(err, "promote manager")
	s.mustAssignManagerDept(manager.ID, rootDept)

	// A：主部门命中管辖树的子孙部门 → 口径 1 可见。
	userA := s.mustCreateUserWithEmail("scope-a@test.com")
	s.setPrimaryDept(userA.ID, childDept)
	groupOther := s.mustCreateGroupNamed("其他组-union-test")
	s.mustSubscribe(userA.ID, groupOther.ID)

	// B：主部门为空/越界，但订阅分组名命中映射 → 口径 2 可见。
	userB := s.mustCreateUserWithEmail("scope-b@test.com")
	groupBig := s.mustCreateGroupNamed(groupBigName)
	s.mustSubscribe(userB.ID, groupBig.ID)

	// C：两条口径都不命中 → 不可见。
	userC := s.mustCreateUserWithEmail("scope-c@test.com")
	s.setPrimaryDept(userC.ID, otherDept)
	s.mustSubscribe(userC.ID, groupOther.ID)

	// D：无任何订阅 → 即使命中也不可见。
	userD := s.mustCreateUserWithEmail("scope-d@test.com")
	s.setPrimaryDept(userD.ID, rootDept)

	ids, page, err := s.repo.ListManagerUserIDs(s.ctx, manager.ID, pagination.PaginationParams{Page: 1, PageSize: 50})
	s.Require().NoError(err, "ListManagerUserIDs")
	s.Require().Equal(int64(2), page.Total, "scope total (A via primary dept, B via group name)")

	got := map[int64]bool{}
	for _, id := range ids {
		got[id] = true
	}
	s.Require().True(got[userA.ID], "A should be visible via primary dept tree")
	s.Require().True(got[userB.ID], "B should be visible via subscription group name")
	s.Require().False(got[userC.ID], "C must stay out of scope")
	s.Require().False(got[userD.ID], "user without subscriptions must stay out of scope")

	for _, tc := range []struct {
		name   string
		userID int64
		want   bool
	}{
		{"A primary-dept child", userA.ID, true},
		{"B group-name fallback", userB.ID, true},
		{"C unrelated dept+group", userC.ID, false},
		{"D no subscription", userD.ID, false},
	} {
		ok, err := s.repo.IsUserInManagerScope(s.ctx, manager.ID, tc.userID)
		s.Require().NoError(err, "IsUserInManagerScope %s", tc.name)
		s.Require().Equal(tc.want, ok, "IsUserInManagerScope %s", tc.name)
	}
}

// TestScope_DeptTreeIncludesGrandchildren 验证管辖部门的子孙（含隔代）也计入 dept_tree。
func (s *ManagerScopeRepoSuite) TestScope_DeptTreeIncludesGrandchildren() {
	const (
		parent = int64(910000101)
		child  = int64(910000102)
		grand  = int64(910000103)
	)

	s.seedDeptGroupMap(`{}`)
	s.mustUpsertDept(parent, 0, "P")
	s.mustUpsertDept(child, parent, "C")
	s.mustUpsertDept(grand, child, "G")

	manager := s.mustCreateUserWithEmail("mgr-tree@test.com")
	_, err := s.client.User.UpdateOneID(manager.ID).SetRole(service.RoleManager).Save(s.ctx)
	s.Require().NoError(err, "promote manager")
	s.mustAssignManagerDept(manager.ID, parent)

	member := s.mustCreateUserWithEmail("tree-member@test.com")
	s.setPrimaryDept(member.ID, grand)
	group := s.mustCreateGroupNamed("tree-group-unique")
	s.mustSubscribe(member.ID, group.ID)

	ok, err := s.repo.IsUserInManagerScope(s.ctx, manager.ID, member.ID)
	s.Require().NoError(err, "IsUserInManagerScope")
	s.Require().True(ok, "grandchild dept must be in manager scope")
}

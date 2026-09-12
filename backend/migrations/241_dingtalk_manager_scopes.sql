-- Department-manager authorization data.
--
-- primary_dept_id is a snapshot of the user's primary DingTalk department. The
-- manager assignment table stores department IDs, not names or subscription
-- groups, so authorization remains stable when a department is renamed.
ALTER TABLE users ADD COLUMN IF NOT EXISTS primary_dept_id BIGINT;

CREATE TABLE IF NOT EXISTS dingtalk_departments (
    dept_id BIGINT PRIMARY KEY,
    parent_id BIGINT NOT NULL DEFAULT 0,
    name VARCHAR(255) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_manager_departments (
    manager_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dept_id BIGINT NOT NULL REFERENCES dingtalk_departments(dept_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (manager_user_id, dept_id)
);

CREATE INDEX IF NOT EXISTS idx_users_primary_dept_id
    ON users (primary_dept_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_manager_departments_dept_id
    ON user_manager_departments (dept_id);

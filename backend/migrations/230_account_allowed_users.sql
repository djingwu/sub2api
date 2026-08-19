-- 账号用户白名单中间表：账号分配给分组后，仅白名单内用户可用该账号。
--
-- account_allowed_users(account_id, user_id) 复合主键。
-- 空白名单 = 分组内所有用户可用（向后兼容）；非空白名单 = 仅指定用户可用。
-- 幂等：重复执行结果一致。
CREATE TABLE IF NOT EXISTS account_allowed_users (
    account_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, user_id),
    CONSTRAINT account_allowed_users_accounts_account
        FOREIGN KEY (account_id) REFERENCES accounts (id),
    CONSTRAINT account_allowed_users_users_user
        FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX IF NOT EXISTS accountalloweduser_user_id
    ON account_allowed_users (user_id);
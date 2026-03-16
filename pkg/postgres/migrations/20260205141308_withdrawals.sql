-- +goose Up
-- +goose StatementBegin
CREATE TABLE users_balances (
    id         SERIAL PRIMARY KEY,
    user_id    UUID   NOT NULL,
    currency   TEXT   NOT NULL,
    amount     INT    NOT NULL DEFAULT 0,
    UNIQUE (user_id, currency)
);

CREATE TABLE idempotency_keys (
    id           SERIAL PRIMARY KEY,
    key          TEXT   NOT NULL,
    user_id      TEXT   NOT NULL,
    request_hash TEXT   NOT NULL,
    status       TEXT   NOT NULL DEFAULT 'pending',
    response     BYTEA,
    status_code  INT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (key, user_id)
);

CREATE TABLE withdrawals (
    operation_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    amount          INT  NOT NULL,
    currency        TEXT NOT NULL,
    destination     TEXT NOT NULL,
    idempotency_key UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ledger_entries (
    id           SERIAL PRIMARY KEY,
    operation_id UUID        NOT NULL REFERENCES withdrawals (operation_id),
    user_id      UUID        NOT NULL,
    amount       INT         NOT NULL,
    currency     TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO users_balances (user_id, currency, amount)
VALUES ('12345678-1234-4123-b234-123456789012', 'USDT', 1000);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE ledger_entries;
DROP TABLE withdrawals;
DROP TABLE idempotency_keys;
DROP TABLE users_balances;
-- +goose StatementEnd

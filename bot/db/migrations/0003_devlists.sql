-- Таблица девов по кошелькам (для авто-открытия / блока в v1).
CREATE TABLE IF NOT EXISTS dev_lists (
    wallet TEXT PRIMARY KEY,
    kind   TEXT NOT NULL DEFAULT 'good', -- 'good' | 'scam'
    note   TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица Twitter-аккаунтов (seed из devs.txt, для будущего Twitter-модуля).
CREATE TABLE IF NOT EXISTS twitter_accounts (
    handle TEXT PRIMARY KEY,
    flag_t BOOLEAN NOT NULL DEFAULT FALSE,
    flag_a BOOLEAN NOT NULL DEFAULT FALSE,
    flag_f BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

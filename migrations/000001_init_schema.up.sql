CREATE TABLE words (
    id   BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL UNIQUE,
    word TEXT NOT NULL UNIQUE
);

CREATE TABLE games (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID NOT NULL UNIQUE,
    user_id           UUID NOT NULL,
    word_id           BIGINT NOT NULL REFERENCES words (id),
    guessed_currently TEXT NOT NULL,
    guesses_remaining INT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX games_user_id_idx ON games (user_id);

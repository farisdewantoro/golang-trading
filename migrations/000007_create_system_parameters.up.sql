CREATE TABLE system_parameters (
    name            VARCHAR(100) PRIMARY KEY,          -- unique key param, contoh: "MAX_RETRY"
    value           JSONB NOT NULL,                    -- isi nilai param (bisa apa saja)
    description     TEXT,                              -- penjelasan param
    deleted_at      TIMESTAMP,                       
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

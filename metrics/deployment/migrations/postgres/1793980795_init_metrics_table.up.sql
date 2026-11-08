BEGIN;

CREATE TABLE IF NOT EXISTS metrics (
    id    TEXT NOT NULL,
    mtype TEXT NOT NULL,
    delta BIGINT,
    value DOUBLE PRECISION,
    hash  TEXT,
    PRIMARY KEY (id, mtype),
    CONSTRAINT metrics_mtype_check CHECK (mtype IN ('gauge', 'counter')),
    CONSTRAINT metrics_payload_check CHECK (
        (mtype = 'counter' AND delta IS NOT NULL AND value IS NULL) OR
        (mtype = 'gauge' AND value IS NOT NULL AND delta IS NULL)
    )
);

END;

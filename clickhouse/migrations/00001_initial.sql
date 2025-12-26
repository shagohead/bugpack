-- +goose Up
-- +goose StatementBegin

CREATE TABLE bug_target
(
	`Timestamp` DateTime CODEC(Delta, LZ4),
	`BugType` String CODEC(LZ4),
	`BugMessage` String CODEC(LZ4),
	`BugHash` UInt64 CODEC(LZ4) DEFAULT xxh3(BugType, BugMessage),
	`Context` Map(LowCardinality(String), String) CODEC(LZ4),
)
ENGINE = Null;

CREATE TABLE bug_events
(
	`BugHash` UInt64 CODEC(LZ4),
	`Timestamp` DateTime CODEC(Delta, LZ4),
	`Context` Map(LowCardinality(String), String) CODEC(LZ4)
)
ENGINE = MergeTree
PARTITION BY toDate(Timestamp)
ORDER BY (BugHash, Timestamp);

CREATE MATERIALIZED VIEW bug_events_mv TO bug_events
(
	`BugHash` UInt64,
	`Timestamp` DateTime,
	`Context` Map(LowCardinality(String), String)
)
AS SELECT `Timestamp`, `BugHash`
FROM bug_target;

CREATE TABLE bug_stats (
)
ENGINE = SummingMergeTree
PARTITION BY toDate(Timestamp)
ORDER BY (BugHash, Timestamp);
-- +goose StatementEnd

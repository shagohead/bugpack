CREATE TABLE IF NOT EXISTS issue_status
(
	`Project` LowCardinality(String) CODEC(ZSTD(1)),
	`IssueHash` UInt64 CODEC(ZSTD(1)),
	`Status` UInt8,
	`Writed` DateTime64(6)
)
ENGINE = ReplacingMergeTree (Writed)
PARTITION BY toDate(Writed)
ORDER BY (Project, IssueHash)
TTL toDateTime(Writed) + toIntervalDay(30);

CREATE TABLE IF NOT EXISTS issue_event
(
	`Project` LowCardinality(String) CODEC(ZSTD(1)),
	`IssueHash` UInt64 DEFAULT xxh3(Level, Message, Exception) CODEC(ZSTD(1)),
	`Level` LowCardinality(String) CODEC(ZSTD(1)),
	`Message` String CODEC(ZSTD(1)),
	`Exception` Tuple(
		ParentHash UInt64,
		Module String,
		Type String,
		Value String,
		Frames Array(Tuple(
			Filename String,
			AbsPath String,
			Module String,
			Function String,
			LineNum UInt32,
			CtxLine String,
			PreCtx Array(String),
			PostCtx Array(String),
			Vars String,
			InApp Bool
		))
	) CODEC(ZSTD(1)),
	`ClientIP` String CODEC(ZSTD(1)),
	`SDK` Tuple(Name LowCardinality(String), Version LowCardinality(String)) CODEC(ZSTD(1)),
	`Platform` LowCardinality(String) CODEC(ZSTD(1)),
	`ServerName` LowCardinality(String) CODEC(ZSTD(1)),
	`Environment` LowCardinality(String) CODEC(ZSTD(1)),
	`Release` LowCardinality(String) CODEC(ZSTD(1)),
	`User` Tuple(ID String, IP String, Email String, Username String, Name String) CODEC(ZSTD(1)),
	`UserData` JSON() CODEC(ZSTD(1)),
	`Context` JSON() CODEC(ZSTD(1)),
	`Tags` Map(String, String) CODEC(ZSTD(1)),
	`Request` Tuple(
		URL String,
		Method LowCardinality(String),
		Data String,
		QueryString String,
		Cookies String,
		Headers Map(LowCardinality(String), String),
		Environ Map(LowCardinality(String), String)
	) CODEC(ZSTD(1)),
	`TraceID` String CODEC(ZSTD(1)),
	`SpanID` String CODEC(ZSTD(1)),
	`Timestamp` DateTime64(6) CODEC(Delta, ZSTD(1)),
	INDEX idx_message Message TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_exc_type Exception.Type TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_exc_value Exception.Value TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_tags_key mapKeys(Tags) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_tags_value mapValues(Tags) TYPE bloom_filter(0.01) GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY (toDate(Timestamp), Project)
PRIMARY KEY (Project, toDate(Timestamp))
ORDER BY (Project, toDate(Timestamp), Timestamp)
TTL toDateTime(Timestamp) + toIntervalDay(15)
SETTINGS ttl_only_drop_parts = 1;

CREATE TABLE IF NOT EXISTS issue_stat
(
	`Project` LowCardinality(String) CODEC(ZSTD(1)),
	`IssueHash` UInt64 CODEC(ZSTD(1)),
	`FirstSeen` SimpleAggregateFunction(min, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`LastSeen` SimpleAggregateFunction(max, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`Count` SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1)),
)
ENGINE = AggregatingMergeTree
PARTITION BY Project
ORDER BY (Project, IssueHash)
TTL toDateTime(LastSeen) + toIntervalDay(30);

CREATE MATERIALIZED VIEW IF NOT EXISTS issue_stat_mv TO issue_stat
(
	`Project` LowCardinality(String),
	`IssueHash` UInt64,
	`FirstSeen` DateTime64(6),
	`LastSeen` DateTime64(6),
	`Count` UInt64
)
AS SELECT
	`Project`,
	`IssueHash`,
	min(Timestamp) `FirstSeen`,
	max(Timestamp) `LastSeen`,
	count() `Count`
FROM issue_event
GROUP BY Project, IssueHash;

CREATE TABLE IF NOT EXISTS issue_client
(
	`Project` LowCardinality(String) CODEC(ZSTD(1)),
	`IssueHash` UInt64 CODEC(ZSTD(1)),
	`ClientIP` String CODEC(ZSTD(1)),
	`FirstSeen` SimpleAggregateFunction(min, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`LastSeen` SimpleAggregateFunction(max, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`Count` SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY Project
ORDER BY (Project, IssueHash, ClientIP)
TTL toDateTime(LastSeen) + toIntervalDay(30);

CREATE MATERIALIZED VIEW IF NOT EXISTS issue_client_mv TO issue_client
(
	`Project` LowCardinality(String),
	`IssueHash` UInt64,
	`ClientIP` String,
	`FirstSeen` DateTime64(6),
	`LastSeen` DateTime64(6),
	`Count` UInt64
)
AS SELECT
	`Project`,
	`IssueHash`,
	`ClientIP`,
	min(Timestamp) `FirstSeen`,
	max(Timestamp) `LastSeen`,
	count() `Count`
FROM issue_event
GROUP BY Project, IssueHash, ClientIP;

CREATE TABLE IF NOT EXISTS issue_user
(
	`Project` LowCardinality(String) CODEC(ZSTD(1)),
	`IssueHash` UInt64 CODEC(ZSTD(1)),
	`User` Tuple(ID String, IP String, Email String, Username String, Name String) CODEC(ZSTD(1)),
	`FirstSeen` SimpleAggregateFunction(min, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`LastSeen` SimpleAggregateFunction(max, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`Count` SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY Project
ORDER BY (Project, IssueHash, User)
TTL toDateTime(LastSeen) + toIntervalDay(30);

CREATE MATERIALIZED VIEW IF NOT EXISTS issue_user_mv TO issue_user
(
	`Project` LowCardinality(String),
	`IssueHash` UInt64,
	`User` Tuple(ID String, IP String, Email String, Username String, Name String),
	`FirstSeen` DateTime64(6),
	`LastSeen` DateTime64(6),
	`Count` UInt64
)
AS SELECT
	`Project`,
	`IssueHash`,
	`User`,
	min(Timestamp) `FirstSeen`,
	max(Timestamp) `LastSeen`,
	count() `Count`
FROM issue_event
GROUP BY Project, IssueHash, User;

CREATE TABLE IF NOT EXISTS issue_tag
(
	`Project` LowCardinality(String) CODEC(ZSTD(1)),
	`IssueHash` UInt64 CODEC(ZSTD(1)),
	`Key` String CODEC(ZSTD(1)),
	`Value` String CODEC(ZSTD(1)),
	`FirstSeen` SimpleAggregateFunction(min, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`LastSeen` SimpleAggregateFunction(max, DateTime64(6)) CODEC(Delta, ZSTD(1)),
	`Count` SimpleAggregateFunction(sum, UInt64) CODEC(ZSTD(1))
)
ENGINE = AggregatingMergeTree
PARTITION BY Project
ORDER BY (Project, IssueHash, Key, Value)
TTL toDateTime(LastSeen) + toIntervalDay(30);

CREATE MATERIALIZED VIEW IF NOT EXISTS issue_tag_mv TO issue_tag
(
	`Project` LowCardinality(String),
	`IssueHash` UInt64,
	`Key` String,
	`Value` String,
	`FirstSeen` DateTime64(6),
	`LastSeen` DateTime64(6),
	`Count` UInt64
)
AS SELECT
	`Project`,
	`IssueHash`,
	`Key`,
	`Value`,
	min(Timestamp) `FirstSeen`,
	max(Timestamp) `LastSeen`,
	count() `Count`
FROM issue_event
ARRAY JOIN Tags.keys AS Key, Tags.values AS Value
GROUP BY Project, IssueHash, Key, Value;

BugPack
=======

Lightweight self-hosted Sentry-compatible bug tracking system with ClickHouse as storage backend.

- Compatible with Sentry API. On the client (project) side use already existen Sentry SDKs.
- Effectivly store large amounts of errors data in ClickHouse DBMS.

### Docker

Docker image published at https://hub.docker.com/r/lastdanmer/bugpack

### Local usage example

```bash
# Rollup ClickHouse & BugPack servers.
docker compose -f example/compose.yaml up -d

# Send sample event using data from tests.
curl -D - -X POST -H "X-Sentry-Auth: Sentry sentry_key=testapi" \
    --data-binary @sentry/testdata/go_panic_0.json \
    $(docker port example-bugpack-1 8080 | head -1)

# Lookup event in database.
#
# issue_event is the main table with full event data.
# But there is another tables which filled by materialized views
# for efficient queries of statistics data.
#
# See schema files in bugpack/chconn/migrations/ directory.
docker exec -it example-clickhouse-1 clickhouse-client \
    -q 'SELECT * FROM issue_event FORMAT Vertical'

# Rolldown servers and all data.
docker compose -f example/compose.yaml down -v
```

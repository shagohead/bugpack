BugPack
=======

Lightweight self-hosted Sentry-compatible bug tracking system with ClickHouse as storage backend.

- Compatible with Sentry API. On the client (project) side use already existen Sentry SDKs.
- Effectivly store large amounts of errors data in ClickHouse DBMS.

### Docker

Docker image published at https://hub.docker.com/r/lastdanmer/bugpack

### Local usage example

```bash
docker compose -f example/compose.yaml up
curl -D - -X POST -H "X-Sentry-Auth: Sentry sentry_key=testapi" --data-binary @sentry/testdata/go_panic_0.json $(docker port example-bugpack-1 8080 | head -1)
docker exec -it example-clickhouse-1 clickhouse-client -q 'SELECT * FROM issue_event FORMAT Vertical'
```

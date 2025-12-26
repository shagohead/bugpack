BugPack
=======

*логотип с гофером держащим сачок и с рюкзаком на спине, из которого выглядывает жук*

Lightweight self-hosted Sentry-compatible bug tracking system with ClickHouse as storage backend.

- Compatible with Sentry API. On the client (project) side use already existen Sentry SDKs.
- Effectivly store large amounts of errors data in ClickHouse DBMS.


# Admin auth (вне MVP)

For administrator authentication use OAuth2 or sidecar proxy.
OAuth2 authenticated users authorized by configured authorization provider.
That provider only needs to check if user identity approved for administration access.

List of currently existing providers:

- `environment variable` - comma separetd list of identities in environment variable;
- `file based provider`
- `external HTTP API `

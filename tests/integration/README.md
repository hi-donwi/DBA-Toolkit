# Integration tests

These tests run the real `dbakit` binary against a live PostgreSQL instance
started with Docker Compose. They are opt-in because they need Docker and a
server; the regular unit suite in `internal/` does not.

## Requirements

- Docker with Compose v2
- Go 1.26

## Run

```sh
make integration
```

or manually:

```sh
docker compose -f tests/integration/docker-compose.yml up -d --wait
go test -tags integration ./tests/integration/ -v
docker compose -f tests/integration/docker-compose.yml down
```

`docker compose up -d --wait` is required: the suite assumes the server is
ready on `127.0.0.1:5433` with user/password/database all `dbakit` /
`dbakit_pw` / `dbakit` (set by the `DBAKIT_DB_*` env overrides in the test
file).

## What it covers

- Real connectivity against PostgreSQL 16 (version, engine, current database).
- Every read-only command (`health`, `diagnose`, `sessions`, `locks`,
  `replication`, `databases`, `config`) emits parseable JSON, includes the
  `CONN-001` connectivity finding, and does not leak the test password.
- A deliberately unreachable endpoint (`--db-port 1`) still exits 0 with a
  single CRITICAL connectivity finding and parseable JSON.
- The binary is built once per suite run with `-tags integration`.

Nothing here writes to the database beyond the container defaults; dbakit
itself stays read-only.
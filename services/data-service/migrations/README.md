# data-service — Database Migrations

Run all migrations with [golang-migrate](https://github.com/golang-migrate/migrate):

```bash
migrate -path ./migrations -database "${DATABASE_URL}" up
```

Or apply manually in order:

```bash
psql $DATABASE_URL -f <migration_file>
```

## Migration Files

- `001_create_kev_entries.down.sql`
- `002_create_kev_entries.up.sql`
- `003_initial_schema.sql`
- `004_create_sync_jobs.up.sql`

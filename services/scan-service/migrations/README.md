# scan-service — Database Migrations

Run all migrations with [golang-migrate](https://github.com/golang-migrate/migrate):

```bash
migrate -path ./migrations -database "${DATABASE_URL}" up
```

Or apply manually in order:

```bash
psql $DATABASE_URL -f <migration_file>
```

## Migration Files

- `001_initial_schema.sql`
- `002_agent_001_initial_schema.sql`
- `003_asset_001_initial_schema.sql`

# notification-service — Database Migrations

Run all migrations with [golang-migrate](https://github.com/golang-migrate/migrate):

```bash
migrate -path ./migrations -database "${DATABASE_URL}" up
```

Or apply manually in order:

```bash
psql $DATABASE_URL -f <migration_file>
```

## Migration Files

- `001_dd_tables.sql`
- `002_create_jira_integrations.up.sql`
- `003_globalcve_001_create_webhooks.down.sql`
- `004_globalcve_001_create_webhooks.up.sql`

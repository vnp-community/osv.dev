# finding-service — Database Migrations

Run all migrations with [golang-migrate](https://github.com/golang-migrate/migrate):

```bash
migrate -path ./migrations -database "${DATABASE_URL}" up
```

Or apply manually in order:

```bash
psql $DATABASE_URL -f <migration_file>
```

## Migration Files

- `001_initial.sql`
- `002_initial.sql`
- `003_orchestrator_001_initial.sql`
- `004_initial_schema.sql`
- `005_sla_001_initial.sql`
- `006_audit_001_initial.sql`

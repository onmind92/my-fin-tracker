.PHONY: db-up db-down migrate-up migrate-down db-backup db-restore backend worker frontend test integration-test build

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate-up:
	cd backend && go run ./cmd/migrate up

migrate-down:
	cd backend && go run ./cmd/migrate down

db-backup:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" && exit 1)
	@test -n "$(BACKUP_FILE)" || (echo "BACKUP_FILE is required, for example backups/finance.dump" && exit 1)
	@mkdir -p "$(dir $(BACKUP_FILE))"
	pg_dump --format=custom --no-owner --file="$(BACKUP_FILE)" "$(DATABASE_URL)"

db-restore:
	@test -n "$(RESTORE_DATABASE_URL)" || (echo "RESTORE_DATABASE_URL must explicitly name the target database" && exit 1)
	@test -n "$(BACKUP_FILE)" || (echo "BACKUP_FILE is required" && exit 1)
	pg_restore --exit-on-error --clean --if-exists --no-owner --dbname="$(RESTORE_DATABASE_URL)" "$(BACKUP_FILE)"

backend:
	cd backend && go run ./cmd/api

worker:
	cd backend && go run ./cmd/worker

frontend:
	cd frontend && npm run dev

test:
	cd backend && go test ./...

integration-test:
	cd backend && go test -tags=integration -v ./integration

build:
	cd backend && go build ./...
	cd frontend && npm run build

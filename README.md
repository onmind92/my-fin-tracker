# Personal Finance Tracker

Личный финансовый трекер на Go, PostgreSQL и React. Текущая версия включает email/password authentication, изоляцию данных пользователей, счета и операции, цели, аналитику, прогноз, шаблоны, recurring worker, экспорт резервной копии и installable PWA.

## Возможности

- регистрация, вход, выход и защищённая cookie-session;
- счета и доходы/расходы с точными суммами в копейках (`BIGINT`);
- категории, финансовые цели, dashboard и история net worth;
- cash-flow, расходы по категориям и прозрачный 24-месячный forecast;
- быстрый ввод, deterministic parser и шаблоны операций;
- регулярные операции в локальном времени пользователя;
- связанные переводы между собственными счетами без искажения cash flow;
- месячные бюджеты с расходом, остатком и прогнозом;
- двухфазный CSV import и безопасный JSON import/merge с remap ID;
- история успешных, пропущенных и неуспешных recurring executions;
- JSON backup и CSV-экспорт операций;
- responsive PWA, standalone-режим на iPhone, safe areas и понятное offline-состояние.

## Архитектура

```text
frontend (React/Vite/PWA)
        │ REST + credentialed fetch + CSRF header
        ▼
backend/cmd/api (chi HTTP API)
        │
        ├── internal/auth       Argon2id, opaque sessions, CSRF
        ├── internal/httpapi    routes, middleware, validation
        ├── internal/store      pgx queries scoped by user_id
        ├── internal/calculation / recurrence / exportdata
        ▼
PostgreSQL ◄── backend/cmd/worker (recurring schedules)
```

API и worker — отдельные процессы, использующие одну PostgreSQL. HTTP handlers не содержат SQL. Владение сущностями проверяется в backend-запросах по authenticated `user_id`; неизвестный или чужой объект возвращается как not found. Старый seed-user сохранён для совместимости локальных данных, но production API больше не авторизует запросы от его имени автоматически.

## Authentication и безопасность

Пароль нормализованного email хранится только как Argon2id hash с уникальной солью. Минимальная длина пароля — 12 символов; искусственных требований к регистру и спецсимволам нет. Email уникален case-insensitively.

При регистрации или входе сервер выдаёт криптографически случайный opaque session token в cookie `finance_session`. В PostgreSQL хранится только SHA-256 hash токена. Cookie имеет `HttpOnly`, настраиваемый `Secure`, `SameSite` и срок жизни; истёкшие сессии удаляет worker. Токены не попадают в `localStorage`.

Для изменяющих запросов используется synchronizer CSRF token: браузер передаёт его в `X-CSRF-Token`, сервер сравнивает hash с текущей session и дополнительно проверяет `Origin`. Login/register также проверяют `Origin` и ограничены простым in-memory rate limiter (10 попыток в минуту на IP и endpoint). CORS разрешает credentials только для точного `FRONTEND_ORIGIN`, без wildcard.

API ограничивает request body до 1 MiB, использует parameterized pgx queries, session expiry, безопасные server timeouts и заголовки `nosniff`, `no-referrer`, CSP `frame-ancestors 'none'`. Пароли, cookies, session tokens, auth headers и raw quick-entry text не логируются.

## Timezone и recurring worker

`users.timezone` содержит проверенное IANA-имя, например `Europe/Moscow`; при регистрации frontend предлагает значение браузера. Timestamps хранятся в UTC, но «сегодня/вчера», месячные границы, dashboard, аналитика и calendar recurrence вычисляются в timezone пользователя.

Recurring schedule хранит `local_time`. Worker загружает timezone владельца, строит следующее локальное календарное время стандартной Go `time` library и переводит его в UTC. Поэтому переходы DST не рассчитываются вручную. Уникальный partial index `(recurring_transaction_id, scheduled_for)` и `ON CONFLICT DO NOTHING` делают повторную обработку идемпотентной.

## Миграции

Старые `000001`–`000004` не изменены. Добавлены:

- `000005_auth_sessions_timezone`: email/password metadata и timezone в `users`, case-insensitive unique email index, таблица `sessions` с hashed tokens и expiry;
- `000006_recurring_timezone`: локальное время запуска recurring schedule.
- `000007_transfers_budgets_recurring_history`: transfers, monthly budgets и recurring execution journal;
- `000008_restore_markers`: маркеры импортированных archive ID для защиты от случайного повтора restore.

Для обеих миграций есть `.down.sql`.

## Локальный запуск

Нужны Go 1.23+, Node.js 20+, npm и PostgreSQL. Самый простой путь к БД — Docker Compose.

```bash
cp .env.example .env
docker compose up -d postgres
cd backend
go run ./cmd/migrate up
go run ./cmd/api
```

В отдельных терминалах:

```bash
cd backend && go run ./cmd/worker
cd frontend && npm install && npm run dev
```

Откройте `http://localhost:5173`, зарегистрируйте пользователя и работайте через UI. API доступен на `http://localhost:8080`; `GET /health` проверяет процесс, `GET /ready` — доступность PostgreSQL без раскрытия DSN или деталей ошибки.

Из корня также доступны `make db-up`, `make migrate-up`, `make backend`, `make worker`, `make frontend`, `make test`, `make integration-test`, `make build`.

## Environment variables

| Variable | Default | Назначение |
|---|---|---|
| `DATABASE_URL` | local Compose DSN | PostgreSQL DSN для API, migrate и worker |
| `APP_ENV` | `development` | имя окружения |
| `HTTP_ADDR` | `:8080` | адрес API; имеет приоритет над `PORT` |
| `PORT` | — | platform-provided port, если `HTTP_ADDR` не задан |
| `FRONTEND_ORIGIN` | `http://localhost:5173` | единственный разрешённый browser origin |
| `COOKIE_SECURE` | `false` | в production обязательно `true` |
| `COOKIE_SAME_SITE` | `lax` | `lax`, `strict`; `none` принимается только вместе с Secure |
| `SESSION_DURATION_DAYS` | `30` | срок server-side session |
| `VITE_API_URL` | `http://localhost:8080` | публичный URL API при frontend build |
| `WORKER_POLL_SECONDS` | `30` | polling interval worker |
| `WORKER_METRICS_ADDR` | `:9091` | адрес metrics endpoint отдельного worker |
| `METRICS_TOKEN` | — | обязательный Bearer token для API/worker `/metrics` |
| `TEST_DATABASE_URL` | — | отдельная PostgreSQL для integration suite |

`SESSION_SECRET` не нужен: каждая сессия использует независимый random opaque token, а в БД хранится только его hash.

## API и export

Public endpoints: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `GET /health`, `GET /ready`.

Все остальные `/api/v1` требуют session cookie. Auth endpoints: `GET/PATCH /auth/me`, `POST /auth/logout`. Финансовые группы: `/accounts`, `/transactions`, `/transfers`, `/budgets`, `/categories`, `/goals`, `/dashboard`, `/analytics/*`, `/transaction-templates`, `/quick-entry/parse`, `/recurring-transactions`.

- `GET /api/v1/export/json` — versioned archive с metadata пользователя, счетами, категориями, операциями, целями, шаблонами и recurring schedules;
- `GET /api/v1/export/transactions.csv` — UTF-8 CSV с BOM и колонками date/type/amount/currency/account/category/description.

Password hash, session rows/tokens и внутренние секреты в export не входят. CSV создаётся стандартным `encoding/csv`, поэтому запятые, кавычки, переносы строк и Unicode экранируются корректно. Кнопки скачивания находятся в Settings.

## Transfers и budgets

Transfer — единая сущность с source/destination account, а не две несвязанные операции. Оба счёта должны принадлежать пользователю, быть активными и иметь одинаковую валюту. Баланс счёта учитывает входящие и исходящие переводы; в total net worth они взаимно компенсируются. Cash flow, expense categories, budget spend и forecast average учитывают только `income`/`expense`.

Budget v1 имеет период `monthly` и expense category. Для активной части текущего месяца вычисляются:

```text
remaining = limit - spent
usage_percent = spent / limit × 100
projected_spend = spent / elapsed_days × total_days
```

При нулевом elapsed period прогноз равен нулю, поэтому API не возвращает `NaN` или Infinity. Dashboard показывает первые три бюджета, полная страница доступна в navigation.

## CSV import и JSON restore

CSV flow двухфазный: `POST /api/v1/import/csv/preview` загружает файл в short-lived memory session на 15 минут; повторный preview с mapping классифицирует строки как `new`, `possible_duplicate` или `invalid`; `POST /api/v1/import/csv/confirm` атомарно записывает только выбранные строки. Поддерживаются comma/semicolon, UTF-8 BOM, signed amounts и несколько форматов даты. Fingerprint использует user, account, timestamp, type, amount и description и является предупреждением, а не абсолютной гарантией.

JSON restore также использует preview/confirm. Это merge, не wipe: категории сопоставляются, а accounts, transactions, transfers, goals, templates, recurring schedules, budgets и execution history получают новые ID. Все ссылки remap-ятся, `user_id` из файла игнорируется, auth/session данные отсутствуют в archive. Повтор того же `archive_id` возвращает conflict.

## Observability

API пишет JSON structured request logs: request ID, method, route pattern, status, duration и внутренний user ID. Request body, email, account/category names и financial descriptions не логируются. Безопасный входящий `X-Request-ID` сохраняется, иначе создаётся UUID; ID возвращается клиенту и включается в server errors.

API `/metrics` и worker `WORKER_METRICS_ADDR/metrics` отдают Prometheus-compatible counters/summaries для HTTP, latency, active sessions, recurring executions и worker failures. Доступ требует `Authorization: Bearer $METRICS_TOKEN`; финансовых или персональных labels нет.

## PostgreSQL backup и recovery

Логический backup создаётся только в явно указанное место, а restore требует отдельный явный target DSN:

```bash
make db-backup DATABASE_URL='postgres://...' BACKUP_FILE='backups/finance-2026-10-03.dump'
make db-restore RESTORE_DATABASE_URL='postgres://target...' BACKUP_FILE='backups/finance-2026-10-03.dump'
```

`db-restore` использует `pg_restore --clean --if-exists --exit-on-error`; это destructive операция для указанной target database. Перед production restore сначала восстановите backup в отдельную БД, примените smoke tests и только затем планируйте переключение. `backups/`, `*.dump` и `*.backup` исключены из Git. Для managed PostgreSQL дополнительно включите platform snapshots/PITR и регулярно проверяйте восстановление.

## PWA и iPhone

Production build создаёт manifest, service worker и собственную SVG-иконку. В Safari на iPhone откройте HTTPS-сайт и выберите **Share → Add to Home Screen**. Интерфейс поддерживает `display: standalone`, dynamic viewport и safe-area отступы для notch/home indicator.

Service worker кэширует только application shell и статические assets. `/api` работает в режиме network-only: приватные auth/financial responses не попадают в PWA cache. Полноценного offline sync нет. Без сети интерфейс показывает offline indicator, а mutation честно сообщает, что данные не сохранены.

## Тесты

```bash
cd backend
go test ./...
go build ./...

cd ../frontend
npm run lint
npm run build
```

Unit tests покрывают Argon2 verify/wrong password, expiry, email/timezone validation, финансовые расчёты, month boundary, today/yesterday parser, DST recurrence, export privacy и CSV escaping.

Integration suite использует отдельную реальную PostgreSQL через `TEST_DATABASE_URL`, создаёт случайную schema, последовательно применяет все `.up.sql` и удаляет schema после теста:

```bash
TEST_DATABASE_URL='postgres://...' go test -tags=integration -v ./integration
```

Без переменной или доступной PostgreSQL suite делает explicit skip и не ломает unit tests. Сценарий дополнительно проверяет transfer balances/net worth/cash flow/delete/isolation/currency, budget timezone boundaries/projection, CSV confirm/duplicates/isolation, export→restore round trip/remap/repeat detection и recurring success/duplicate/failure history.

## Production deployment outline

Рекомендуемая схема: frontend на Vercel/Railway static service, API и worker как два Railway service, PostgreSQL как managed Railway database.

1. Создать production PostgreSQL и передать один `DATABASE_URL` API, migrate release-step и worker.
2. Перед rollout применить `go run ./cmd/migrate up`; не запускать destructive down migration.
3. API: задать `APP_ENV=production`, platform `PORT`, точный HTTPS `FRONTEND_ORIGIN`, `COOKIE_SECURE=true`, session duration и health checks: liveness `/health`, readiness `/ready`.
4. Frontend: собрать с production `VITE_API_URL`; `frontend/vercel.json` даёт SPA fallback для `/login`, `/register` и protected routes.
5. Worker: использовать тот же `DATABASE_URL`, настроить `WORKER_POLL_SECONDS`; одного instance достаточно для личного deployment.
6. Предпочтительны frontend/API поддомены одного registrable domain: тогда подходит `COOKIE_SAME_SITE=lax`. Если они реально cross-site, нужны HTTPS, `COOKIE_SECURE=true`, `COOKIE_SAME_SITE=none`; при этом следует учесть browser third-party-cookie policy и лучше настроить same-site reverse proxy/custom domains.
7. На frontend host отдельно задать CSP под фактические asset/API origins; backend CSP защищает только API responses.
8. Настроить TLS, backup/retention PostgreSQL, secret storage платформы, log retention и мониторинг `/ready`.
9. Сгенерировать длинный `METRICS_TOKEN`, подключить scraper с Bearer header и не публиковать worker metrics port наружу.
10. Настроить ежедневные platform snapshots/PITR и периодический проверяемый `pg_dump` restore drill.

API обрабатывает SIGTERM через graceful HTTP shutdown с 10-секундным timeout. Worker прекращает polling по SIGTERM, а pgx pool закрывается при выходе.

## Ограничения и следующий этап

Пока нет email verification, password reset, OAuth, bank-specific import adapters, FX transfers, банковских API, инвестиций, OCR/AI, WebSocket, Redis, notifications и native mobile app. CSV duplicate fingerprint остаётся эвристикой; import sessions живут в памяти одного API instance и не предназначены для горизонтально распределённого confirm flow.

Логичный Milestone 6: password reset/email verification, пользовательские категории, bank-specific CSV profiles, правила категоризации без AI, budget rollover, transfer editing UX и автоматизированные recovery drills/alerting.

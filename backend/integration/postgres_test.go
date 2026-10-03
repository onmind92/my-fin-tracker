//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"finance-tracker/backend/internal/auth"
	"finance-tracker/backend/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresMilestonesEndToEnd(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration suite skipped")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Ping(ctx); err != nil {
		t.Skipf("PostgreSQL unavailable: %v", err)
	}
	schema := "finance_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+identifier+" CASCADE") })
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	applyMigrations(t, ctx, db)
	repository := store.New(db)
	authService := auth.New(repository, 30*24*time.Hour)
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	userA, tokensA, err := authService.Register(ctx, "a@example.com", "correct horse battery staple", nil, "Europe/Moscow", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = authService.Register(ctx, "A@example.com", "correct horse battery staple", nil, "UTC", now); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate email accepted: %v", err)
	}
	if _, _, err = authService.Login(ctx, "a@example.com", "wrong password", now); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, _, err = authService.Login(ctx, "a@example.com", "correct horse battery staple", now); err != nil {
		t.Fatal(err)
	}
	if _, err = authService.Authenticate(ctx, tokensA.SessionToken, now); err != nil {
		t.Fatal(err)
	}
	if err = authService.Logout(ctx, tokensA.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, err = authService.Authenticate(ctx, tokensA.SessionToken, now); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatal("logout did not invalidate session")
	}
	userB, _, err := authService.Register(ctx, "b@example.com", "another correct long password", nil, "UTC", now)
	if err != nil {
		t.Fatal(err)
	}
	account, err := repository.CreateAccount(ctx, userA.ID, store.CreateAccountInput{Name: "Main", Type: "card", Currency: "RUB", OpeningBalanceMinor: 50_000_00})
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := repository.CreateTransaction(ctx, userA.ID, store.CreateTransactionInput{AccountID: account.ID, Type: "income", AmountMinor: 20_000_00, OccurredAt: &now})
	if err != nil {
		t.Fatal(err)
	}
	account, err = repository.GetAccount(ctx, userA.ID, account.ID)
	if err != nil || account.CurrentBalanceMinor != 70_000_00 {
		t.Fatalf("balance=%d err=%v", account.CurrentBalanceMinor, err)
	}
	savings, err := repository.CreateAccount(ctx, userA.ID, store.CreateAccountInput{Name: "Savings", Type: "savings", Currency: "RUB", OpeningBalanceMinor: 10_000_00})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE accounts SET created_at=$2 WHERE id IN ($1,$3)`, account.ID, now.AddDate(0, -2, 0), savings.ID); err != nil {
		t.Fatal(err)
	}
	beforeTransfer := account.CurrentBalanceMinor + savings.CurrentBalanceMinor
	transfer, err := repository.CreateTransfer(ctx, userA.ID, store.TransferInput{FromAccountID: account.ID, ToAccountID: savings.ID, AmountMinor: 5_000_00, OccurredAt: &now})
	if err != nil {
		t.Fatal(err)
	}
	sourceAfter, _ := repository.GetAccount(ctx, userA.ID, account.ID)
	destinationAfter, _ := repository.GetAccount(ctx, userA.ID, savings.ID)
	if sourceAfter.CurrentBalanceMinor != 65_000_00 || destinationAfter.CurrentBalanceMinor != 15_000_00 || sourceAfter.CurrentBalanceMinor+destinationAfter.CurrentBalanceMinor != beforeTransfer {
		t.Fatalf("transfer balances source=%d destination=%d", sourceAfter.CurrentBalanceMinor, destinationAfter.CurrentBalanceMinor)
	}
	accountHistory, err := repository.AccountHistory(ctx, userA.ID, account.ID, "1m", now)
	if err != nil || len(accountHistory) == 0 || accountHistory[len(accountHistory)-1].BalanceMinor != 65_000_00 {
		t.Fatalf("account history after transfer: %+v %v", accountHistory, err)
	}
	netHistory, err := repository.NetWorthHistory(ctx, userA.ID, "1m", now)
	if err != nil || len(netHistory) == 0 || netHistory[len(netHistory)-1].NetWorthMinor != beforeTransfer {
		t.Fatalf("net worth changed by transfer: %+v %v", netHistory, err)
	}
	flow, err := repository.CashFlow(ctx, userA.ID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(flow) != 1 || flow[0].IncomeMinor != 20_000_00 {
		t.Fatalf("transfer changed cash flow: %+v %v", flow, err)
	}
	if _, err = repository.CreateTransfer(ctx, userB.ID, store.TransferInput{FromAccountID: account.ID, ToAccountID: savings.ID, AmountMinor: 1}); err == nil {
		t.Fatal("cross-user transfer allowed")
	}
	if err = repository.DeleteTransfer(ctx, userA.ID, transfer.ID); err != nil {
		t.Fatal(err)
	}
	afterDelete, _ := repository.GetAccount(ctx, userA.ID, account.ID)
	if afterDelete.CurrentBalanceMinor != 70_000_00 {
		t.Fatal("deleting transfer did not restore balance")
	}
	dashboard, err := repository.Dashboard(ctx, userA.ID, now.In(time.FixedZone("MSK", 3*3600)))
	if err != nil || dashboard.NetWorthMinor != 80_000_00 {
		t.Fatalf("dashboard=%+v err=%v", dashboard, err)
	}
	goal, err := repository.CreateGoal(ctx, userA.ID, store.CreateGoalInput{Name: "Reserve", Type: "current", TargetAmountMinor: 100_000_00, LinkedAccountID: &account.ID}, now)
	if err != nil || goal.CurrentAmountMinor != 70_000_00 || goal.ProgressPercent != 70 {
		t.Fatalf("live goal calculation=%+v err=%v", goal, err)
	}
	if _, err = repository.GetAccount(ctx, userB.ID, account.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("cross-user account read allowed")
	}
	if _, err = repository.UpdateTransaction(ctx, userB.ID, transaction.ID, store.UpdateTransactionInput{AmountMinor: pointer(int64(1))}); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("cross-user transaction patch allowed")
	}
	if err = repository.DeleteTransaction(ctx, userB.ID, transaction.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("cross-user transaction delete allowed")
	}
	goalInput := store.CreateGoalInput{Name: "Foreign", Type: "current", TargetAmountMinor: 100, LinkedAccountID: &account.ID}
	if _, err = repository.CreateGoal(ctx, userB.ID, goalInput, now); err == nil {
		t.Fatal("cross-user goal link allowed")
	}
	templateInput := store.TemplateInput{Name: "Foreign", TransactionType: "expense", AccountID: &account.ID}
	if _, err = repository.CreateTemplate(ctx, userB.ID, templateInput); err == nil {
		t.Fatal("cross-user template link allowed")
	}
	recurringInput := store.RecurringInput{TransactionType: "expense", AmountMinor: 100, AccountID: account.ID, Frequency: "monthly", Interval: 1, StartDate: "2026-10-01", LocalTime: "09:00"}
	if _, err = repository.CreateRecurring(ctx, userB.ID, recurringInput); err == nil {
		t.Fatal("cross-user recurring link allowed")
	}
	otherDashboard, err := repository.Dashboard(ctx, userB.ID, now)
	if err != nil || otherDashboard.NetWorthMinor != 0 {
		t.Fatal("dashboard mixed users")
	}
	history, err := repository.NetWorthHistory(ctx, userB.ID, "1m", now)
	if err != nil {
		t.Fatal(err)
	}
	foodID := "10000000-0000-0000-0000-000000000001"
	boundary := time.Date(2026, 9, 30, 22, 30, 0, 0, time.UTC)
	if _, err = repository.CreateTransaction(ctx, userA.ID, store.CreateTransactionInput{AccountID: account.ID, CategoryID: &foodID, Type: "expense", AmountMinor: 3_000_00, OccurredAt: &boundary}); err != nil {
		t.Fatal(err)
	}
	salaryID := "20000000-0000-0000-0000-000000000001"
	if _, err = repository.CreateTransaction(ctx, userA.ID, store.CreateTransactionInput{AccountID: account.ID, CategoryID: &salaryID, Type: "income", AmountMinor: 9_000_00, OccurredAt: &boundary}); err != nil {
		t.Fatal(err)
	}
	budget, err := repository.CreateBudget(ctx, userA.ID, store.BudgetInput{CategoryID: foodID, AmountMinor: 10_000_00, Period: "monthly", StartDate: "2026-10-01"}, now.In(time.FixedZone("MSK", 3*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if budget.SpentMinor != 3_000_00 || budget.ProjectedSpendMinor <= budget.SpentMinor {
		t.Fatalf("budget calculation: %+v", budget)
	}
	budgetTransfer, err := repository.CreateTransfer(ctx, userA.ID, store.TransferInput{FromAccountID: account.ID, ToAccountID: savings.ID, AmountMinor: 500_00, OccurredAt: &boundary})
	if err != nil {
		t.Fatal(err)
	}
	budgetAfterTransfer, err := repository.GetBudget(ctx, userA.ID, budget.ID, now.In(time.FixedZone("MSK", 3*3600)))
	if err != nil || budgetAfterTransfer.SpentMinor != 3_000_00 {
		t.Fatalf("transfer entered budget spend: %+v %v", budgetAfterTransfer, err)
	}
	if err = repository.DeleteTransfer(ctx, userA.ID, budgetTransfer.ID); err != nil {
		t.Fatal(err)
	}
	for _, point := range history {
		if point.NetWorthMinor != 0 {
			t.Fatal("analytics mixed users")
		}
	}
	exportA, err := repository.Export(ctx, userA, now)
	if err != nil || len(exportA.Data.Transactions) != 3 {
		t.Fatalf("user A export: transactions=%d err=%v", len(exportA.Data.Transactions), err)
	}
	exportB, err := repository.Export(ctx, userB, now)
	if err != nil || len(exportB.Data.Transactions) != 0 || len(exportB.Data.Accounts) != 0 {
		t.Fatalf("export mixed users: accounts=%d transactions=%d err=%v", len(exportB.Data.Accounts), len(exportB.Data.Transactions), err)
	}
	recurring, err := repository.CreateRecurring(ctx, userA.ID, store.RecurringInput{TransactionType: "expense", AmountMinor: 500, AccountID: account.ID, Frequency: "daily", Interval: 1, StartDate: "2026-10-01", LocalTime: "09:00"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.ProcessDueRecurrences(ctx, now, 100)
	if err != nil || first.Generated == 0 {
		t.Fatalf("first worker: %+v %v", first, err)
	}
	scheduled := time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	_, err = db.Exec(ctx, `UPDATE recurring_transactions SET next_run_at=$2 WHERE id=$1`, recurring.ID, scheduled)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.ProcessDueRecurrences(ctx, now, 100)
	if err != nil || second.Duplicates == 0 {
		t.Fatalf("idempotency not enforced: %+v %v", second, err)
	}
	executions, err := repository.ListRecurringExecutions(ctx, userA.ID, recurring.ID, 20)
	if err != nil || len(executions) < 2 || executions[0].Status != "success" {
		t.Fatalf("recurring execution history: %+v %v", executions, err)
	}
	archive, err := repository.Export(ctx, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	userC, _, err := authService.Register(ctx, "restore@example.com", "restore password long enough", nil, "UTC", now)
	if err != nil {
		t.Fatal(err)
	}
	var sessionsBefore int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id=$1`, userC.ID).Scan(&sessionsBefore); err != nil {
		t.Fatal(err)
	}
	restored, err := repository.RestoreArchive(ctx, userC.ID, archive.ArchiveID, archive.Data)
	if err != nil || restored.Accounts != len(archive.Data.Accounts) || restored.Transactions != len(archive.Data.Transactions) || restored.Goals != len(archive.Data.Goals) || restored.Recurring != len(archive.Data.RecurringTransactions) {
		t.Fatalf("restore round trip: %+v %v", restored, err)
	}
	var sessionsAfter int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id=$1`, userC.ID).Scan(&sessionsAfter); err != nil || sessionsAfter != sessionsBefore {
		t.Fatal("restore imported or changed sessions")
	}
	if _, _, err = authService.Login(ctx, "restore@example.com", "restore password long enough", now); err != nil {
		t.Fatal("restore changed authentication")
	}
	if _, err = repository.RestoreArchive(ctx, userC.ID, archive.ArchiveID, archive.Data); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("repeated archive not detected: %v", err)
	}
	restoredAccounts, err := repository.ListAccounts(ctx, userC.ID, false)
	if err != nil || len(restoredAccounts) != len(archive.Data.Accounts) || restoredAccounts[0].ID == archive.Data.Accounts[0].ID {
		t.Fatal("restore IDs were not remapped")
	}
	importDescription := "Imported"
	importItem := store.ImportTransaction{AccountID: account.ID, Type: "expense", AmountMinor: 777, Description: &importDescription, OccurredAt: now.Add(time.Hour)}
	if _, err = repository.ImportTransactions(ctx, userB.ID, []store.ImportTransaction{importItem}); err == nil {
		t.Fatal("cross-user CSV import target allowed")
	}
	if count, importErr := repository.ImportTransactions(ctx, userA.ID, []store.ImportTransaction{importItem}); importErr != nil || count != 1 {
		t.Fatalf("CSV confirm: %d %v", count, importErr)
	}
	duplicate, err := repository.IsPossibleDuplicate(ctx, userA.ID, importItem)
	if err != nil || !duplicate {
		t.Fatal("CSV duplicate fingerprint not detected")
	}
	if _, err = db.Exec(ctx, `ALTER TABLE accounts DROP CONSTRAINT accounts_currency_check`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE accounts SET currency='USD' WHERE id=$1`, savings.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.CreateTransfer(ctx, userA.ID, store.TransferInput{FromAccountID: account.ID, ToAccountID: savings.ID, AmountMinor: 100}); err == nil {
		t.Fatal("different-currency transfer allowed")
	}
	if _, err = db.Exec(ctx, `UPDATE accounts SET currency='RUB' WHERE id=$1`, savings.ID); err != nil {
		t.Fatal(err)
	}
	skipped, err := repository.CreateRecurring(ctx, userA.ID, store.RecurringInput{TransactionType: "expense", AmountMinor: 100, AccountID: savings.ID, Frequency: "daily", Interval: 1, StartDate: "2026-10-02", LocalTime: "09:00"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE accounts SET is_active=false WHERE id=$1`, savings.ID); err != nil {
		t.Fatal(err)
	}
	skippedResult, err := repository.ProcessDueRecurrences(ctx, now, 100)
	if err != nil || skippedResult.Paused == 0 {
		t.Fatalf("inactive recurring not skipped: %+v %v", skippedResult, err)
	}
	skippedHistory, err := repository.ListRecurringExecutions(ctx, userA.ID, skipped.ID, 20)
	if err != nil || len(skippedHistory) != 1 || skippedHistory[0].Status != "skipped" {
		t.Fatalf("skipped execution history: %+v %v", skippedHistory, err)
	}
	failing, err := repository.CreateRecurring(ctx, userA.ID, store.RecurringInput{TransactionType: "expense", AmountMinor: 100, AccountID: account.ID, Frequency: "daily", Interval: 1, StartDate: "2026-10-02", LocalTime: "09:00"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE users SET timezone='Invalid/Zone' WHERE id=$1`, userA.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ProcessDueRecurrences(ctx, now, 100); err == nil {
		t.Fatal("worker failure scenario did not fail")
	}
	if _, err = db.Exec(ctx, `UPDATE users SET timezone='Europe/Moscow' WHERE id=$1`, userA.ID); err != nil {
		t.Fatal(err)
	}
	failureHistory, err := repository.ListRecurringExecutions(ctx, userA.ID, failing.ID, 20)
	if err != nil || len(failureHistory) != 1 || failureHistory[0].Status != "failed" {
		t.Fatalf("failed execution history: %+v %v", failureHistory, err)
	}
}

func applyMigrations(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) < 8 {
		t.Fatalf("expected all migrations, got %d", len(files))
	}
	for _, file := range files {
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(file), err)
		}
	}
}
func pointer[T any](value T) *T { return &value }

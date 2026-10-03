package calculation

import (
	"testing"
	"time"
)

func TestGlobalGoalProgress(t *testing.T) {
	got := CalculateGoal(GoalInput{CurrentAmountMinor: 347_540_00, TargetAmountMinor: 1_000_000_00, Now: day("2026-09-01")})
	if got.ProgressPercent != 34.75 || got.RemainingAmountMinor != 652_460_00 || got.Reached {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestManualCurrentGoalProgress(t *testing.T) {
	got := CalculateGoal(GoalInput{CurrentAmountMinor: 25_000_00, TargetAmountMinor: 100_000_00, Now: day("2026-09-01")})
	if got.ProgressPercent != 25 {
		t.Fatalf("got %.2f", got.ProgressPercent)
	}
}

func TestLinkedAccountGoalProgress(t *testing.T) {
	linkedBalance := int64(75_000_00)
	got := CalculateGoal(GoalInput{CurrentAmountMinor: linkedBalance, TargetAmountMinor: 100_000_00, Now: day("2026-09-01")})
	if got.ProgressPercent != 75 || got.RemainingAmountMinor != 25_000_00 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestRequiredMonthlyPace(t *testing.T) {
	deadline := day("2027-03-01")
	got := CalculateGoal(GoalInput{CurrentAmountMinor: 40_000_00, TargetAmountMinor: 100_000_00, Deadline: &deadline, Now: day("2026-09-01")})
	if got.RequiredMonthlyAmountMinor == nil || *got.RequiredMonthlyAmountMinor != 10_000_00 {
		t.Fatalf("unexpected pace: %+v", got.RequiredMonthlyAmountMinor)
	}
}

func TestReachedTargetCapsProgress(t *testing.T) {
	got := CalculateGoal(GoalInput{CurrentAmountMinor: 120_000, TargetAmountMinor: 100_000, Now: day("2026-09-01")})
	if !got.Reached || got.ProgressPercent != 100 || got.RemainingAmountMinor != 0 {
		t.Fatalf("unexpected reached result: %+v", got)
	}
}

func TestForecastPositiveAndNegativeCashFlow(t *testing.T) {
	positive := Forecast(1_000_000, 100_000, 6, 6, day("2026-09-01"))
	if positive.Forecast[5].NetWorthMinor != 1_600_000 {
		t.Fatalf("positive forecast: %+v", positive.Forecast[5])
	}
	negative := Forecast(1_000_000, -100_000, 6, 6, day("2026-09-01"))
	if negative.Forecast[5].NetWorthMinor != 400_000 {
		t.Fatalf("negative forecast: %+v", negative.Forecast[5])
	}
}

func TestForecastInsufficientHistory(t *testing.T) {
	got := Forecast(1_000_000, 50_000, 12, 2, day("2026-09-01"))
	if got.RequestedWindowMonths != 12 || got.ActualWindowMonths != 2 {
		t.Fatalf("unexpected windows: %+v", got)
	}
}

func TestNetWorthHistoryCalculation(t *testing.T) {
	accounts := []LedgerAccount{
		{ID: "a", OpeningBalanceMinor: 500_000, CreatedAt: day("2026-01-10")},
		{ID: "b", OpeningBalanceMinor: 200_000, CreatedAt: day("2026-02-15")},
	}
	transactions := []LedgerTransaction{
		{AccountID: "a", Type: "income", AmountMinor: 100_000, OccurredAt: day("2026-01-20")},
		{AccountID: "a", Type: "expense", AmountMinor: 50_000, OccurredAt: day("2026-03-01")},
	}
	if got := NetWorthAt(accounts, transactions, day("2026-01-31")); got != 600_000 {
		t.Fatalf("January got %d", got)
	}
	if got := NetWorthAt(accounts, transactions, day("2026-02-28")); got != 800_000 {
		t.Fatalf("February got %d", got)
	}
}

func TestExpensesByCategory(t *testing.T) {
	food := "food"
	transport := "transport"
	tx := []LedgerTransaction{
		{Type: "expense", AmountMinor: 300, CategoryID: &food, CategoryName: "Food", OccurredAt: day("2026-01-10")},
		{Type: "expense", AmountMinor: 100, CategoryID: &transport, CategoryName: "Transport", OccurredAt: day("2026-01-11")},
		{Type: "income", AmountMinor: 900, CategoryName: "Salary", OccurredAt: day("2026-01-12")},
	}
	got := ExpensesByCategory(tx, day("2026-01-01"), day("2026-02-01"))
	if len(got) != 2 || got[0].CategoryName != "Food" || got[0].SharePercent != 75 {
		t.Fatalf("unexpected categories: %+v", got)
	}
	empty := ExpensesByCategory(nil, day("2026-01-01"), day("2026-02-01"))
	if len(empty) != 0 {
		t.Fatalf("expected empty result")
	}
}

func TestPastDeadlineHasNoFalseRequiredPace(t *testing.T) {
	deadline := day("2026-01-01")
	got := CalculateGoal(GoalInput{CurrentAmountMinor: 0, TargetAmountMinor: 100, Deadline: &deadline, Now: day("2026-02-01")})
	if got.DaysRemaining == nil || *got.DaysRemaining >= 0 || got.RequiredMonthlyAmountMinor != nil {
		t.Fatalf("unexpected past deadline result: %+v", got)
	}
}

func TestEditingTransactionChangesAccountBalance(t *testing.T) {
	accounts := []LedgerAccount{{ID: "old", OpeningBalanceMinor: 100_000, CreatedAt: day("2026-01-01")}, {ID: "new", OpeningBalanceMinor: 200_000, CreatedAt: day("2026-01-01")}}
	before := []LedgerTransaction{{AccountID: "old", Type: "expense", AmountMinor: 10_000, OccurredAt: day("2026-01-02")}}
	after := []LedgerTransaction{{AccountID: "new", Type: "expense", AmountMinor: 10_000, OccurredAt: day("2026-01-02")}}
	if got := NetWorthAt(accounts[:1], before, day("2026-01-31")); got != 90_000 {
		t.Fatalf("old balance %d", got)
	}
	if got := NetWorthAt(accounts[1:], after, day("2026-01-31")); got != 190_000 {
		t.Fatalf("new balance %d", got)
	}
}

func TestDeletingTransactionChangesNetWorth(t *testing.T) {
	accounts := []LedgerAccount{{ID: "a", OpeningBalanceMinor: 100_000, CreatedAt: day("2026-01-01")}}
	transaction := []LedgerTransaction{{AccountID: "a", Type: "expense", AmountMinor: 25_000, OccurredAt: day("2026-01-02")}}
	if before := NetWorthAt(accounts, transaction, day("2026-01-31")); before != 75_000 {
		t.Fatal(before)
	}
	if after := NetWorthAt(accounts, nil, day("2026-01-31")); after != 100_000 {
		t.Fatal(after)
	}
}

func TestCashFlowUsesUserTimezoneMonthBoundary(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Skip(err)
	}
	transactionTime := time.Date(2026, 3, 31, 21, 30, 0, 0, time.UTC)
	points := MonthlyCashFlow([]LedgerTransaction{{Type: "income", AmountMinor: 100, OccurredAt: transactionTime}}, time.Date(2026, 4, 1, 0, 0, 0, 0, location), time.Date(2026, 5, 1, 0, 0, 0, 0, location))
	if len(points) != 1 || points[0].IncomeMinor != 100 {
		t.Fatalf("timezone boundary failed: %+v", points)
	}
}

func TestTransfersMoveAccountBalanceButNotNetWorthOrCashFlow(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	accounts := []LedgerAccount{{ID: "a", OpeningBalanceMinor: 1000}, {ID: "b", OpeningBalanceMinor: 200}}
	items := []LedgerTransaction{{AccountID: "a", Type: "transfer_out", AmountMinor: 300, OccurredAt: now}, {AccountID: "b", Type: "transfer_in", AmountMinor: 300, OccurredAt: now}}
	if got := NetWorthAt(accounts, items, now); got != 1200 {
		t.Fatalf("net worth changed: %d", got)
	}
	flow := MonthlyCashFlow(items, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if len(flow) != 1 || flow[0].CashFlowMinor != 0 || flow[0].IncomeMinor != 0 || flow[0].ExpenseMinor != 0 {
		t.Fatalf("transfer entered cash flow: %+v", flow)
	}
}
func TestBudgetProjectionHandlesZeroElapsed(t *testing.T) {
	remaining, usage, projected := BudgetProgress(10_000, 2_500, 31, 0)
	if remaining != 7_500 || usage != 25 || projected != 0 {
		t.Fatalf("unexpected projection: %d %.2f %d", remaining, usage, projected)
	}
	_, _, projected = BudgetProgress(10_000, 2_500, 30, 5)
	if projected != 15_000 {
		t.Fatalf("projection=%d", projected)
	}
}

func day(value string) time.Time { parsed, _ := time.Parse("2006-01-02", value); return parsed }

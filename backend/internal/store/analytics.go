package store

import (
	"context"
	"time"

	"finance-tracker/backend/internal/calculation"
	"finance-tracker/backend/internal/domain"
)

func (s *Store) loadLedger(ctx context.Context, userID string, activeAccountsOnly bool) ([]calculation.LedgerAccount, []calculation.LedgerTransaction, error) {
	accountQuery := `SELECT id,opening_balance_minor,created_at FROM accounts WHERE user_id=$1`
	if activeAccountsOnly {
		accountQuery += ` AND is_active=true`
	}
	rows, err := s.db.Query(ctx, accountQuery, userID)
	if err != nil {
		return nil, nil, err
	}
	accounts := make([]calculation.LedgerAccount, 0)
	for rows.Next() {
		var item calculation.LedgerAccount
		if err := rows.Scan(&item.ID, &item.OpeningBalanceMinor, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, err
		}
		accounts = append(accounts, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	txQuery := `SELECT t.account_id,t.type,t.amount_minor,t.occurred_at,t.category_id,COALESCE(c.name,'Без категории') FROM transactions t JOIN accounts a ON a.id=t.account_id LEFT JOIN categories c ON c.id=t.category_id WHERE t.user_id=$1 AND a.user_id=$1`
	if activeAccountsOnly {
		txQuery += ` AND a.is_active=true`
	}
	rows, err = s.db.Query(ctx, txQuery, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	transactions := make([]calculation.LedgerTransaction, 0)
	for rows.Next() {
		var item calculation.LedgerTransaction
		if err := rows.Scan(&item.AccountID, &item.Type, &item.AmountMinor, &item.OccurredAt, &item.CategoryID, &item.CategoryName); err != nil {
			return nil, nil, err
		}
		transactions = append(transactions, item)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	rows.Close()
	transferQuery := `SELECT from_account_id,to_account_id,amount_minor,occurred_at FROM transfers WHERE user_id=$1`
	transferRows, err := s.db.Query(ctx, transferQuery, userID)
	if err != nil {
		return nil, nil, err
	}
	defer transferRows.Close()
	for transferRows.Next() {
		var fromID, toID string
		var amount int64
		var occurred time.Time
		if err = transferRows.Scan(&fromID, &toID, &amount, &occurred); err != nil {
			return nil, nil, err
		}
		transactions = append(transactions, calculation.LedgerTransaction{AccountID: fromID, Type: "transfer_out", AmountMinor: amount, OccurredAt: occurred}, calculation.LedgerTransaction{AccountID: toID, Type: "transfer_in", AmountMinor: amount, OccurredAt: occurred})
	}
	return accounts, transactions, transferRows.Err()
}

func (s *Store) NetWorthHistory(ctx context.Context, userID, rangeName string, now time.Time) ([]domain.NetWorthPoint, error) {
	accounts, transactions, err := s.loadLedger(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	points := historyDates(rangeName, now, accounts)
	return calculation.NetWorthHistory(accounts, transactions, points), nil
}

func historyDates(rangeName string, now time.Time, accounts []calculation.LedgerAccount) []time.Time {
	now = endOfDay(now)
	points := make([]time.Time, 0)
	switch rangeName {
	case "1m":
		for p := endOfDay(now.AddDate(0, -1, 0)); !p.After(now); p = p.AddDate(0, 0, 1) {
			points = append(points, p)
		}
	case "3m":
		for p := endOfDay(now.AddDate(0, -3, 0)); p.Before(now); p = p.AddDate(0, 0, 7) {
			points = append(points, p)
		}
		points = append(points, now)
	default:
		months := 6
		if rangeName == "1y" {
			months = 12
		}
		start := monthStart(now).AddDate(0, -months+1, 0)
		if rangeName == "all" {
			start = monthStart(now)
			for _, a := range accounts {
				candidate := monthStart(a.CreatedAt.In(now.Location()))
				if candidate.Before(start) {
					start = candidate
				}
			}
		}
		for p := start; p.Before(monthStart(now)); p = p.AddDate(0, 1, 0) {
			points = append(points, p.AddDate(0, 1, 0).Add(-time.Nanosecond))
		}
		points = append(points, now)
	}
	return points
}

func (s *Store) CashFlow(ctx context.Context, userID string, from, to time.Time) ([]domain.CashFlowPoint, error) {
	_, transactions, err := s.loadLedger(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	return calculation.MonthlyCashFlow(transactions, from, to), nil
}

func (s *Store) ExpensesByCategory(ctx context.Context, userID string, from, to time.Time) ([]domain.CategoryExpense, error) {
	_, transactions, err := s.loadLedger(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	return calculation.ExpensesByCategory(transactions, from, to), nil
}

func (s *Store) Forecast(ctx context.Context, userID string, requested int, now time.Time) (domain.Forecast, error) {
	accounts, transactions, err := s.loadLedger(ctx, userID, true)
	if err != nil {
		return domain.Forecast{}, err
	}
	current := calculation.NetWorthAt(accounts, transactions, now)
	currentMonth := monthStart(now)
	earliest := currentMonth
	for _, account := range accounts {
		candidate := monthStart(account.CreatedAt.In(now.Location()))
		if account.CreatedAt.After(candidate) {
			candidate = candidate.AddDate(0, 1, 0)
		}
		if candidate.Before(earliest) {
			earliest = candidate
		}
	}
	available := monthDifference(earliest, currentMonth)
	actual := requested
	if actual > available {
		actual = available
	}
	if actual < 0 {
		actual = 0
	}
	average := int64(0)
	if actual > 0 {
		points := calculation.MonthlyCashFlow(transactions, currentMonth.AddDate(0, -actual, 0), currentMonth)
		average = calculation.AverageCashFlow(points)
	}
	return calculation.Forecast(current, average, requested, actual, now), nil
}

func monthDifference(from, to time.Time) int {
	return (to.Year()-from.Year())*12 + int(to.Month()-from.Month())
}
func monthStart(value time.Time) time.Time {
	y, m, _ := value.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, value.Location())
}
func endOfDay(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), value.Location())
}

package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"finance-tracker/backend/internal/calculation"
	"finance-tracker/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("resource not found")
var ErrConflict = errors.New("resource already exists")

type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

type CreateAccountInput struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Currency            string `json:"currency"`
	OpeningBalanceMinor int64  `json:"opening_balance_minor"`
}

type UpdateAccountInput struct {
	Name     *string `json:"name"`
	Type     *string `json:"type"`
	IsActive *bool   `json:"is_active"`
}

func validateAccount(name, accountType, currency string) error {
	if strings.TrimSpace(name) == "" {
		return ValidationError{"name must not be empty"}
	}
	valid := map[string]bool{"card": true, "cash": true, "savings": true, "investment": true, "other": true}
	if !valid[accountType] {
		return ValidationError{"invalid account type"}
	}
	if currency != "RUB" {
		return ValidationError{"currency must be RUB"}
	}
	return nil
}

const accountSelect = `
SELECT a.id, a.user_id, a.name, a.type, a.currency, a.opening_balance_minor,
       a.opening_balance_minor
       + COALESCE(SUM(CASE WHEN t.type='income' THEN t.amount_minor ELSE -t.amount_minor END), 0)
       + COALESCE((SELECT SUM(amount_minor) FROM transfers WHERE user_id=a.user_id AND to_account_id=a.id AND occurred_at<=now()),0)
       - COALESCE((SELECT SUM(amount_minor) FROM transfers WHERE user_id=a.user_id AND from_account_id=a.id AND occurred_at<=now()),0) AS current_balance_minor,
       a.is_active, a.created_at, a.updated_at
FROM accounts a LEFT JOIN transactions t ON t.account_id=a.id AND t.occurred_at<=now()
WHERE a.user_id=$1`

func scanAccount(row pgx.Row) (domain.Account, error) {
	var a domain.Account
	err := row.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.Currency, &a.OpeningBalanceMinor, &a.CurrentBalanceMinor, &a.IsActive, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) ListAccounts(ctx context.Context, userID string, activeOnly bool) ([]domain.Account, error) {
	q := accountSelect
	if activeOnly {
		q += " AND a.is_active=true"
	}
	q += " GROUP BY a.id ORDER BY a.created_at"
	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Account, 0)
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (s *Store) GetAccount(ctx context.Context, userID, id string) (domain.Account, error) {
	return scanAccount(s.db.QueryRow(ctx, accountSelect+" AND a.id=$2 GROUP BY a.id", userID, id))
}

func (s *Store) CreateAccount(ctx context.Context, userID string, in CreateAccountInput) (domain.Account, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if err := validateAccount(in.Name, in.Type, in.Currency); err != nil {
		return domain.Account{}, err
	}
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO accounts (id,user_id,name,type,currency,opening_balance_minor) VALUES ($1,$2,$3,$4,$5,$6)`, id, userID, in.Name, in.Type, in.Currency, in.OpeningBalanceMinor)
	if err != nil {
		return domain.Account{}, err
	}
	return s.GetAccount(ctx, userID, id)
}

func (s *Store) UpdateAccount(ctx context.Context, userID, id string, in UpdateAccountInput) (domain.Account, error) {
	current, err := s.GetAccount(ctx, userID, id)
	if err != nil {
		return domain.Account{}, err
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Type != nil {
		current.Type = *in.Type
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if err := validateAccount(current.Name, current.Type, current.Currency); err != nil {
		return domain.Account{}, err
	}
	_, err = s.db.Exec(ctx, `UPDATE accounts SET name=$3,type=$4,is_active=$5,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, current.Name, current.Type, current.IsActive)
	if err != nil {
		return domain.Account{}, err
	}
	return s.GetAccount(ctx, userID, id)
}

type CreateTransactionInput struct {
	AccountID   string     `json:"account_id"`
	CategoryID  *string    `json:"category_id"`
	Type        string     `json:"type"`
	AmountMinor int64      `json:"amount_minor"`
	Description *string    `json:"description"`
	OccurredAt  *time.Time `json:"occurred_at"`
}

type UpdateTransactionInput struct {
	AccountID   *string    `json:"account_id"`
	CategoryID  *string    `json:"category_id"`
	Type        *string    `json:"type"`
	AmountMinor *int64     `json:"amount_minor"`
	Description *string    `json:"description"`
	OccurredAt  *time.Time `json:"occurred_at"`
}

type TransactionFilter struct {
	Limit, Offset   int
	AccountID, Type string
}

func validateTransaction(in CreateTransactionInput) error {
	if in.AmountMinor <= 0 {
		return ValidationError{"amount_minor must be greater than zero"}
	}
	if in.Type != "income" && in.Type != "expense" {
		return ValidationError{"type must be income or expense"}
	}
	if _, err := uuid.Parse(in.AccountID); err != nil {
		return ValidationError{"account_id must be a UUID"}
	}
	if in.CategoryID != nil {
		if _, err := uuid.Parse(*in.CategoryID); err != nil {
			return ValidationError{"category_id must be a UUID"}
		}
	}
	return nil
}

func (s *Store) validateRelations(ctx context.Context, userID string, in CreateTransactionInput) error {
	var active bool
	if err := s.db.QueryRow(ctx, `SELECT is_active FROM accounts WHERE id=$1 AND user_id=$2`, in.AccountID, userID).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
		return ValidationError{"account does not exist or is not owned by user"}
	} else if err != nil {
		return err
	}
	if !active {
		return ValidationError{"account is inactive"}
	}
	if in.CategoryID != nil {
		var categoryType string
		err := s.db.QueryRow(ctx, `SELECT type FROM categories WHERE id=$1 AND (user_id IS NULL OR user_id=$2)`, *in.CategoryID, userID).Scan(&categoryType)
		if errors.Is(err, pgx.ErrNoRows) {
			return ValidationError{"category does not exist or is not available to user"}
		}
		if err != nil {
			return err
		}
		if categoryType != in.Type {
			return ValidationError{"category type must match transaction type"}
		}
	}
	return nil
}

const transactionSelect = `SELECT t.id,t.user_id,t.account_id,a.name,t.category_id,c.name,c.icon,t.type,t.amount_minor,t.description,t.occurred_at,t.created_at,t.updated_at,t.recurring_transaction_id,t.scheduled_for
FROM transactions t JOIN accounts a ON a.id=t.account_id LEFT JOIN categories c ON c.id=t.category_id`

func scanTransaction(row pgx.Row) (domain.Transaction, error) {
	var t domain.Transaction
	err := row.Scan(&t.ID, &t.UserID, &t.AccountID, &t.AccountName, &t.CategoryID, &t.CategoryName, &t.CategoryIcon, &t.Type, &t.AmountMinor, &t.Description, &t.OccurredAt, &t.CreatedAt, &t.UpdatedAt, &t.RecurringTransactionID, &t.ScheduledFor)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (s *Store) GetTransaction(ctx context.Context, userID, id string) (domain.Transaction, error) {
	return scanTransaction(s.db.QueryRow(ctx, transactionSelect+` WHERE t.user_id=$1 AND t.id=$2`, userID, id))
}

func (s *Store) ListTransactions(ctx context.Context, userID string, f TransactionFilter) ([]domain.Transaction, error) {
	q := transactionSelect + ` WHERE t.user_id=$1 AND ($2='' OR t.account_id::text=$2) AND ($3='' OR t.type=$3) ORDER BY t.occurred_at DESC,t.created_at DESC LIMIT $4 OFFSET $5`
	rows, err := s.db.Query(ctx, q, userID, f.AccountID, f.Type, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Store) CreateTransaction(ctx context.Context, userID string, in CreateTransactionInput) (domain.Transaction, error) {
	if err := validateTransaction(in); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.validateRelations(ctx, userID, in); err != nil {
		return domain.Transaction{}, err
	}
	when := time.Now().UTC()
	if in.OccurredAt != nil {
		when = *in.OccurredAt
	}
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO transactions (id,user_id,account_id,category_id,type,amount_minor,description,occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, id, userID, in.AccountID, in.CategoryID, in.Type, in.AmountMinor, in.Description, when)
	if err != nil {
		return domain.Transaction{}, err
	}
	return s.GetTransaction(ctx, userID, id)
}

func (s *Store) UpdateTransaction(ctx context.Context, userID, id string, in UpdateTransactionInput) (domain.Transaction, error) {
	cur, err := s.GetTransaction(ctx, userID, id)
	if err != nil {
		return cur, err
	}
	merged := CreateTransactionInput{AccountID: cur.AccountID, CategoryID: cur.CategoryID, Type: cur.Type, AmountMinor: cur.AmountMinor, Description: cur.Description, OccurredAt: &cur.OccurredAt}
	if in.AccountID != nil {
		merged.AccountID = *in.AccountID
	}
	if in.CategoryID != nil {
		merged.CategoryID = in.CategoryID
	}
	if in.Type != nil {
		merged.Type = *in.Type
	}
	if in.AmountMinor != nil {
		merged.AmountMinor = *in.AmountMinor
	}
	if in.Description != nil {
		merged.Description = in.Description
	}
	if in.OccurredAt != nil {
		merged.OccurredAt = in.OccurredAt
	}
	if err = validateTransaction(merged); err != nil {
		return cur, err
	}
	if err = s.validateRelations(ctx, userID, merged); err != nil {
		return cur, err
	}
	_, err = s.db.Exec(ctx, `UPDATE transactions SET account_id=$3,category_id=$4,type=$5,amount_minor=$6,description=$7,occurred_at=$8,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, merged.AccountID, merged.CategoryID, merged.Type, merged.AmountMinor, merged.Description, merged.OccurredAt)
	if err != nil {
		return cur, err
	}
	return s.GetTransaction(ctx, userID, id)
}

func (s *Store) DeleteTransaction(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM transactions WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	rows, err := s.db.Query(ctx, `SELECT id,user_id,name,type,icon,created_at FROM categories WHERE user_id IS NULL OR user_id=$1 ORDER BY type,name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Category, 0)
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Icon, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) Dashboard(ctx context.Context, userID string, now time.Time) (domain.Dashboard, error) {
	accounts, err := s.ListAccounts(ctx, userID, true)
	if err != nil {
		return domain.Dashboard{}, err
	}
	result := domain.Dashboard{Currency: "RUB", Accounts: accounts, RecentTransactions: make([]domain.Transaction, 0)}
	for _, a := range accounts {
		result.NetWorthMinor += a.CurrentBalanceMinor
	}
	monthFrom := monthStart(now)
	err = s.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount_minor) FILTER (WHERE type='income'),0),COALESCE(SUM(amount_minor) FILTER (WHERE type='expense'),0) FROM transactions WHERE user_id=$1 AND occurred_at>=$2 AND occurred_at<=$3`, userID, monthFrom, now).Scan(&result.Month.IncomeMinor, &result.Month.ExpenseMinor)
	if err != nil {
		return result, fmt.Errorf("month summary: %w", err)
	}
	result.Month.CashFlowMinor = result.Month.IncomeMinor - result.Month.ExpenseMinor
	ledgerAccounts, ledgerTransactions, err := s.loadLedger(ctx, userID, true)
	if err != nil {
		return result, err
	}
	monthBase := calculation.NetWorthAt(ledgerAccounts, ledgerTransactions, monthStart(now).Add(-time.Nanosecond))
	result.NetWorthChange.AmountMinor = result.NetWorthMinor - monthBase
	if monthBase != 0 {
		value := math.Round(float64(result.NetWorthChange.AmountMinor)*10000/float64(monthBase)) / 100
		result.NetWorthChange.Percent = &value
	}
	goals, err := s.ListGoals(ctx, userID, "active", now)
	if err != nil {
		return result, err
	}
	if len(goals) > 3 {
		goals = goals[:3]
	}
	result.ActiveGoals = goals
	forecast, err := s.Forecast(ctx, userID, 6, now)
	if err != nil {
		return result, err
	}
	result.Forecast.AverageMonthlyGrowthMinor = forecast.AverageMonthlyGrowthMinor
	result.Forecast.SixMonthsMinor = result.NetWorthMinor + forecast.AverageMonthlyGrowthMinor*6
	result.Forecast.TwelveMonthsMinor = result.NetWorthMinor + forecast.AverageMonthlyGrowthMinor*12
	result.RecentTransactions, err = s.ListTransactions(ctx, userID, TransactionFilter{Limit: 8})
	if err == nil {
		result.BudgetPreview, err = s.ListBudgets(ctx, userID, now)
		if len(result.BudgetPreview) > 3 {
			result.BudgetPreview = result.BudgetPreview[:3]
		}
	}
	return result, err
}

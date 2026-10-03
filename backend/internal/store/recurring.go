package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"finance-tracker/backend/internal/domain"
	"finance-tracker/backend/internal/recurrence"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RecurringInput struct {
	TransactionType string  `json:"transaction_type"`
	AmountMinor     int64   `json:"amount_minor"`
	AccountID       string  `json:"account_id"`
	CategoryID      *string `json:"category_id"`
	Description     *string `json:"description"`
	Frequency       string  `json:"frequency"`
	Interval        int     `json:"interval"`
	StartDate       string  `json:"start_date"`
	EndDate         *string `json:"end_date"`
	LocalTime       string  `json:"local_time"`
	IsActive        *bool   `json:"is_active"`
}

const recurringSelect = `SELECT r.id,r.user_id,r.transaction_type,r.amount_minor,r.account_id,a.name,r.category_id,c.name,r.description,r.frequency,r.interval_value,r.start_date,r.next_run_at,r.end_date,to_char(r.local_time,'HH24:MI'),r.is_active,r.created_at,r.updated_at FROM recurring_transactions r JOIN accounts a ON a.id=r.account_id LEFT JOIN categories c ON c.id=r.category_id WHERE r.user_id=$1`

func scanRecurring(row pgx.Row) (domain.RecurringTransaction, error) {
	var item domain.RecurringTransaction
	err := row.Scan(&item.ID, &item.UserID, &item.TransactionType, &item.AmountMinor, &item.AccountID, &item.AccountName, &item.CategoryID, &item.CategoryName, &item.Description, &item.Frequency, &item.Interval, &item.StartDate, &item.NextRunAt, &item.EndDate, &item.LocalTime, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func validateRecurring(in RecurringInput) (time.Time, *time.Time, error) {
	if in.TransactionType != "expense" && in.TransactionType != "income" {
		return time.Time{}, nil, ValidationError{"transaction_type must be expense or income"}
	}
	if in.AmountMinor <= 0 {
		return time.Time{}, nil, ValidationError{"amount_minor must be greater than zero"}
	}
	if in.AccountID == "" {
		return time.Time{}, nil, ValidationError{"account_id is required"}
	}
	if in.Interval < 1 {
		return time.Time{}, nil, ValidationError{"interval must be at least 1"}
	}
	valid := map[string]bool{"daily": true, "weekly": true, "monthly": true, "yearly": true}
	if !valid[in.Frequency] {
		return time.Time{}, nil, ValidationError{"invalid frequency"}
	}
	start, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return time.Time{}, nil, ValidationError{"start_date must use YYYY-MM-DD"}
	}
	var end *time.Time
	if in.EndDate != nil && strings.TrimSpace(*in.EndDate) != "" {
		value, err := time.Parse("2006-01-02", *in.EndDate)
		if err != nil {
			return time.Time{}, nil, ValidationError{"end_date must use YYYY-MM-DD"}
		}
		if value.Before(start) {
			return time.Time{}, nil, ValidationError{"end_date must not be before start_date"}
		}
		end = &value
	}
	if in.LocalTime == "" {
		in.LocalTime = "09:00"
	}
	if _, err := time.Parse("15:04", in.LocalTime); err != nil {
		return time.Time{}, nil, ValidationError{"local_time must use HH:MM"}
	}
	return start, end, nil
}
func (s *Store) ListRecurring(ctx context.Context, userID string) ([]domain.RecurringTransaction, error) {
	rows, err := s.db.Query(ctx, recurringSelect+` ORDER BY r.is_active DESC,r.next_run_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RecurringTransaction, 0)
	for rows.Next() {
		item, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) GetRecurring(ctx context.Context, userID, id string) (domain.RecurringTransaction, error) {
	return scanRecurring(s.db.QueryRow(ctx, recurringSelect+` AND r.id=$2`, userID, id))
}
func (s *Store) CreateRecurring(ctx context.Context, userID string, in RecurringInput) (domain.RecurringTransaction, error) {
	if in.LocalTime == "" {
		in.LocalTime = "09:00"
	}
	start, end, err := validateRecurring(in)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	accountID := &in.AccountID
	if err = s.validateOptionalReferences(ctx, userID, in.TransactionType, accountID, in.CategoryID, true); err != nil {
		return domain.RecurringTransaction{}, err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	id := uuid.NewString()
	timezone, err := s.GetUserTimezone(ctx, userID)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	nextRun, err := localSchedule(start, in.LocalTime, timezone)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO recurring_transactions(id,user_id,transaction_type,amount_minor,account_id,category_id,description,frequency,interval_value,start_date,next_run_at,end_date,local_time,is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, id, userID, in.TransactionType, in.AmountMinor, in.AccountID, in.CategoryID, in.Description, in.Frequency, in.Interval, start, nextRun, end, in.LocalTime, active)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	return s.GetRecurring(ctx, userID, id)
}
func (s *Store) UpdateRecurring(ctx context.Context, userID, id string, in RecurringInput) (domain.RecurringTransaction, error) {
	if in.LocalTime == "" {
		in.LocalTime = "09:00"
	}
	current, err := s.GetRecurring(ctx, userID, id)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	start, end, err := validateRecurring(in)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	accountID := &in.AccountID
	if err = s.validateOptionalReferences(ctx, userID, in.TransactionType, accountID, in.CategoryID, true); err != nil {
		return domain.RecurringTransaction{}, err
	}
	active := current.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	nextRun := current.NextRunAt
	if !sameDate(start, current.StartDate) || in.Frequency != current.Frequency || in.Interval != current.Interval || in.LocalTime != current.LocalTime {
		timezone, timezoneErr := s.GetUserTimezone(ctx, userID)
		if timezoneErr != nil {
			return domain.RecurringTransaction{}, timezoneErr
		}
		nextRun, err = localSchedule(start, in.LocalTime, timezone)
		if err != nil {
			return domain.RecurringTransaction{}, err
		}
	}
	tag, err := s.db.Exec(ctx, `UPDATE recurring_transactions SET transaction_type=$3,amount_minor=$4,account_id=$5,category_id=$6,description=$7,frequency=$8,interval_value=$9,start_date=$10,next_run_at=$11,end_date=$12,local_time=$13,is_active=$14,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, in.TransactionType, in.AmountMinor, in.AccountID, in.CategoryID, in.Description, in.Frequency, in.Interval, start, nextRun, end, in.LocalTime, active)
	if err != nil {
		return domain.RecurringTransaction{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.RecurringTransaction{}, ErrNotFound
	}
	return s.GetRecurring(ctx, userID, id)
}
func (s *Store) DisableRecurring(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `UPDATE recurring_transactions SET is_active=false,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) ListRecurringExecutions(ctx context.Context, userID, recurringID string, limit int) ([]domain.RecurringExecution, error) {
	if _, err := s.GetRecurring(ctx, userID, recurringID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.Query(ctx, `SELECT id,recurring_transaction_id,user_id,scheduled_for,executed_at,transaction_id,status,error_message FROM recurring_executions WHERE user_id=$1 AND recurring_transaction_id=$2 ORDER BY scheduled_for DESC LIMIT $3`, userID, recurringID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.RecurringExecution{}
	for rows.Next() {
		var item domain.RecurringExecution
		if err = rows.Scan(&item.ID, &item.RecurringTransactionID, &item.UserID, &item.ScheduledFor, &item.ExecutedAt, &item.TransactionID, &item.Status, &item.ErrorMessage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) ListAllRecurringExecutions(ctx context.Context, userID string) ([]domain.RecurringExecution, error) {
	rows, err := s.db.Query(ctx, `SELECT id,recurring_transaction_id,user_id,scheduled_for,executed_at,transaction_id,status,error_message FROM recurring_executions WHERE user_id=$1 ORDER BY scheduled_for DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.RecurringExecution{}
	for rows.Next() {
		var item domain.RecurringExecution
		if err = rows.Scan(&item.ID, &item.RecurringTransactionID, &item.UserID, &item.ScheduledFor, &item.ExecutedAt, &item.TransactionID, &item.Status, &item.ErrorMessage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) recordRecurringFailure(ctx context.Context, itemID, userID string, scheduled time.Time, message string) {
	if len(message) > 240 {
		message = message[:240]
	}
	_, _ = s.db.Exec(ctx, `INSERT INTO recurring_executions(id,recurring_transaction_id,user_id,scheduled_for,status,error_message) VALUES($1,$2,$3,$4,'failed',$5) ON CONFLICT (recurring_transaction_id,scheduled_for) DO UPDATE SET executed_at=now(),status='failed',error_message=EXCLUDED.error_message`, uuid.NewString(), itemID, userID, scheduled, message)
}
func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
func localSchedule(date time.Time, clock, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, ValidationError{"invalid user timezone"}
	}
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, ValidationError{"local_time must use HH:MM"}
	}
	y, m, d := date.Date()
	return time.Date(y, m, d, parsed.Hour(), parsed.Minute(), 0, 0, location).UTC(), nil
}

type ProcessResult struct {
	Generated  int
	Duplicates int
	Paused     int
}

func (s *Store) ProcessDueRecurrences(ctx context.Context, now time.Time, limit int) (ProcessResult, error) {
	result := ProcessResult{}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT r.id,r.user_id,r.transaction_type,r.amount_minor,r.account_id,r.category_id,r.description,r.frequency,r.interval_value,r.start_date,r.next_run_at,r.end_date,to_char(r.local_time,'HH24:MI'),r.is_active,a.is_active,u.timezone FROM recurring_transactions r JOIN accounts a ON a.id=r.account_id JOIN users u ON u.id=r.user_id WHERE r.is_active=true AND r.next_run_at<=$1 ORDER BY r.next_run_at FOR UPDATE OF r SKIP LOCKED LIMIT $2`, now, limit)
	if err != nil {
		return result, err
	}
	type due struct {
		ID, UserID, Kind, AccountID string
		Amount                      int64
		CategoryID, Description     *string
		Frequency                   string
		Interval                    int
		Start, Next                 time.Time
		End                         *time.Time
		LocalTime, Timezone         string
		Active, AccountActive       bool
	}
	items := make([]due, 0)
	for rows.Next() {
		var item due
		if err := rows.Scan(&item.ID, &item.UserID, &item.Kind, &item.Amount, &item.AccountID, &item.CategoryID, &item.Description, &item.Frequency, &item.Interval, &item.Start, &item.Next, &item.End, &item.LocalTime, &item.Active, &item.AccountActive, &item.Timezone); err != nil {
			rows.Close()
			return result, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	for _, item := range items {
		location, locationErr := time.LoadLocation(item.Timezone)
		if locationErr != nil {
			_ = tx.Rollback(ctx)
			s.recordRecurringFailure(ctx, item.ID, item.UserID, item.Next, "invalid user timezone")
			return result, locationErr
		}
		localStart, localErr := localSchedule(item.Start, item.LocalTime, item.Timezone)
		if localErr != nil {
			return result, localErr
		}
		localStart = localStart.In(location)
		localNext := item.Next.In(location)
		localNow := now.In(location)
		var localEnd *time.Time
		if item.End != nil {
			y, m, d := item.End.Date()
			value := time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), location)
			localEnd = &value
		}
		plan := recurrence.BuildPlan(recurrence.Rule{Frequency: item.Frequency, Interval: item.Interval, StartDate: localStart, NextRunAt: localNext, EndDate: localEnd, Active: item.Active, AccountActive: item.AccountActive}, localNow, 120)
		if plan.Pause {
			_, _ = tx.Exec(ctx, `INSERT INTO recurring_executions(id,recurring_transaction_id,user_id,scheduled_for,status,error_message) VALUES($1,$2,$3,$4,'skipped',$5) ON CONFLICT (recurring_transaction_id,scheduled_for) DO NOTHING`, uuid.NewString(), item.ID, item.UserID, item.Next, "schedule or account inactive")
			_, err = tx.Exec(ctx, `UPDATE recurring_transactions SET is_active=false,updated_at=now() WHERE id=$1`, item.ID)
			if err != nil {
				return result, err
			}
			result.Paused++
			continue
		}
		next := localNext
		for _, scheduled := range plan.Dates {
			id := uuid.NewString()
			scheduledUTC := scheduled.UTC()
			tag, execErr := tx.Exec(ctx, `INSERT INTO transactions(id,user_id,account_id,category_id,type,amount_minor,description,occurred_at,recurring_transaction_id,scheduled_for) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$8) ON CONFLICT (recurring_transaction_id,scheduled_for) WHERE recurring_transaction_id IS NOT NULL AND scheduled_for IS NOT NULL DO NOTHING`, id, item.UserID, item.AccountID, item.CategoryID, item.Kind, item.Amount, item.Description, scheduledUTC, item.ID)
			if execErr != nil {
				_ = tx.Rollback(ctx)
				s.recordRecurringFailure(ctx, item.ID, item.UserID, scheduledUTC, "transaction generation failed")
				return result, execErr
			}
			if tag.RowsAffected() == 1 {
				result.Generated++
				_, err = tx.Exec(ctx, `INSERT INTO recurring_executions(id,recurring_transaction_id,user_id,scheduled_for,transaction_id,status) VALUES($1,$2,$3,$4,$5,'success') ON CONFLICT (recurring_transaction_id,scheduled_for) DO NOTHING`, uuid.NewString(), item.ID, item.UserID, scheduledUTC, id)
				if err != nil {
					return result, err
				}
			} else {
				result.Duplicates++
			}
			next = recurrence.Next(next, item.Start, item.Frequency, item.Interval)
		}
		active := true
		if localEnd != nil && next.After(*localEnd) {
			active = false
		}
		if len(plan.Dates) == 0 {
			active = false
		}
		if _, err = tx.Exec(ctx, `UPDATE recurring_transactions SET next_run_at=$2,is_active=$3,updated_at=now() WHERE id=$1`, item.ID, next.UTC(), active); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

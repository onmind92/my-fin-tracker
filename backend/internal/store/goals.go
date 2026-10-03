package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"finance-tracker/backend/internal/calculation"
	"finance-tracker/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CreateGoalInput struct {
	Name                     string  `json:"name"`
	Type                     string  `json:"type"`
	TargetAmountMinor        int64   `json:"target_amount_minor"`
	Deadline                 *string `json:"deadline"`
	LinkedAccountID          *string `json:"linked_account_id"`
	ManualCurrentAmountMinor *int64  `json:"manual_current_amount_minor"`
	Status                   string  `json:"status"`
}

type UpdateGoalInput struct {
	Name                     *string `json:"name"`
	Type                     *string `json:"type"`
	TargetAmountMinor        *int64  `json:"target_amount_minor"`
	Deadline                 *string `json:"deadline"`
	LinkedAccountID          *string `json:"linked_account_id"`
	ManualCurrentAmountMinor *int64  `json:"manual_current_amount_minor"`
	Status                   *string `json:"status"`
}

type rawGoal struct {
	domain.Goal
	LinkedCurrent *int64
}

const goalSelect = `SELECT g.id,g.user_id,g.name,g.type,g.target_amount_minor,g.deadline,g.linked_account_id,a.name,g.manual_current_amount_minor,g.status,g.created_at,g.updated_at,
CASE WHEN g.linked_account_id IS NULL THEN NULL ELSE a.opening_balance_minor + COALESCE((SELECT SUM(CASE WHEN t.type='income' THEN t.amount_minor ELSE -t.amount_minor END) FROM transactions t WHERE t.account_id=a.id AND t.occurred_at<=now()),0) END
FROM goals g LEFT JOIN accounts a ON a.id=g.linked_account_id AND a.user_id=g.user_id WHERE g.user_id=$1`

func scanRawGoal(row pgx.Row) (rawGoal, error) {
	var item rawGoal
	err := row.Scan(&item.ID, &item.UserID, &item.Name, &item.Type, &item.TargetAmountMinor, &item.Deadline, &item.LinkedAccountID, &item.LinkedAccountName, &item.ManualCurrentAmountMinor, &item.Status, &item.CreatedAt, &item.UpdatedAt, &item.LinkedCurrent)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func enrichGoal(item rawGoal, netWorth, average int64, now time.Time) domain.Goal {
	current := int64(0)
	if item.Type == "global" {
		current = netWorth
	} else if item.LinkedCurrent != nil {
		current = *item.LinkedCurrent
	} else if item.ManualCurrentAmountMinor != nil {
		current = *item.ManualCurrentAmountMinor
	}
	calc := calculation.CalculateGoal(calculation.GoalInput{CurrentAmountMinor: current, TargetAmountMinor: item.TargetAmountMinor, Deadline: item.Deadline, AverageMonthlyGrowthMinor: average, Now: now})
	item.CurrentAmountMinor = current
	item.ProgressPercent = calc.ProgressPercent
	item.RemainingAmountMinor = calc.RemainingAmountMinor
	item.DaysRemaining = calc.DaysRemaining
	item.RequiredMonthlyAmountMinor = calc.RequiredMonthlyAmountMinor
	item.EstimatedCompletionDate = calc.EstimatedCompletionDate
	item.ExpectedMonthsToComplete = calc.ExpectedMonthsToComplete
	item.Reached = calc.Reached
	return item.Goal
}

func (s *Store) goalContext(ctx context.Context, userID string, now time.Time) (int64, int64, error) {
	forecast, err := s.Forecast(ctx, userID, 6, now)
	if err != nil {
		return 0, 0, err
	}
	return forecast.CurrentNetWorthMinor, forecast.AverageMonthlyGrowthMinor, nil
}

func (s *Store) ListGoals(ctx context.Context, userID, status string, now time.Time) ([]domain.Goal, error) {
	netWorth, average, err := s.goalContext(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	query := goalSelect + ` AND ($2='' OR g.status=$2) ORDER BY CASE g.status WHEN 'active' THEN 0 WHEN 'completed' THEN 1 ELSE 2 END,g.created_at DESC`
	rows, err := s.db.Query(ctx, query, userID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Goal, 0)
	for rows.Next() {
		raw, err := scanRawGoal(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, enrichGoal(raw, netWorth, average, now))
	}
	return items, rows.Err()
}

func (s *Store) GetGoal(ctx context.Context, userID, id string, now time.Time) (domain.Goal, error) {
	raw, err := scanRawGoal(s.db.QueryRow(ctx, goalSelect+` AND g.id=$2`, userID, id))
	if err != nil {
		return domain.Goal{}, err
	}
	netWorth, average, err := s.goalContext(ctx, userID, now)
	if err != nil {
		return domain.Goal{}, err
	}
	return enrichGoal(raw, netWorth, average, now), nil
}

func parseGoalDeadline(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := time.Parse("2006-01-02", *raw)
	if err != nil {
		return nil, ValidationError{"deadline must use YYYY-MM-DD"}
	}
	return &value, nil
}

func validateGoal(in CreateGoalInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return ValidationError{"name must not be empty"}
	}
	if in.Type != "global" && in.Type != "current" {
		return ValidationError{"type must be global or current"}
	}
	if in.TargetAmountMinor <= 0 {
		return ValidationError{"target_amount_minor must be greater than zero"}
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Status != "active" && in.Status != "completed" && in.Status != "archived" {
		return ValidationError{"invalid goal status"}
	}
	if in.Type == "global" && (in.LinkedAccountID != nil || in.ManualCurrentAmountMinor != nil) {
		return ValidationError{"global goal cannot have a progress source"}
	}
	if in.Type == "current" && ((in.LinkedAccountID == nil) == (in.ManualCurrentAmountMinor == nil)) {
		return ValidationError{"current goal requires exactly one progress source"}
	}
	if in.ManualCurrentAmountMinor != nil && *in.ManualCurrentAmountMinor < 0 {
		return ValidationError{"manual_current_amount_minor must not be negative"}
	}
	if in.LinkedAccountID != nil {
		if _, err := uuid.Parse(*in.LinkedAccountID); err != nil {
			return ValidationError{"linked_account_id must be a UUID"}
		}
	}
	_, err := parseGoalDeadline(in.Deadline)
	return err
}

func (s *Store) validateGoalAccount(ctx context.Context, userID string, accountID *string) error {
	if accountID == nil {
		return nil
	}
	var active bool
	err := s.db.QueryRow(ctx, `SELECT is_active FROM accounts WHERE id=$1 AND user_id=$2`, *accountID, userID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ValidationError{"linked account does not exist or is not owned by user"}
	}
	if err != nil {
		return err
	}
	if !active {
		return ValidationError{"linked account is archived"}
	}
	return nil
}

func (s *Store) CreateGoal(ctx context.Context, userID string, in CreateGoalInput, now time.Time) (domain.Goal, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Status == "" {
		in.Status = "active"
	}
	if err := validateGoal(in); err != nil {
		return domain.Goal{}, err
	}
	if err := s.validateGoalAccount(ctx, userID, in.LinkedAccountID); err != nil {
		return domain.Goal{}, err
	}
	deadline, err := parseGoalDeadline(in.Deadline)
	if err != nil {
		return domain.Goal{}, err
	}
	id := uuid.NewString()
	_, err = s.db.Exec(ctx, `INSERT INTO goals(id,user_id,name,type,target_amount_minor,deadline,linked_account_id,manual_current_amount_minor,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, userID, in.Name, in.Type, in.TargetAmountMinor, deadline, in.LinkedAccountID, in.ManualCurrentAmountMinor, in.Status)
	if err != nil {
		return domain.Goal{}, err
	}
	return s.GetGoal(ctx, userID, id, now)
}

func (s *Store) UpdateGoal(ctx context.Context, userID, id string, in UpdateGoalInput, now time.Time) (domain.Goal, error) {
	current, err := s.GetGoal(ctx, userID, id, now)
	if err != nil {
		return domain.Goal{}, err
	}
	deadlineRaw := (*string)(nil)
	if current.Deadline != nil {
		v := current.Deadline.Format("2006-01-02")
		deadlineRaw = &v
	}
	merged := CreateGoalInput{Name: current.Name, Type: current.Type, TargetAmountMinor: current.TargetAmountMinor, Deadline: deadlineRaw, LinkedAccountID: current.LinkedAccountID, ManualCurrentAmountMinor: current.ManualCurrentAmountMinor, Status: current.Status}
	if in.Name != nil {
		merged.Name = *in.Name
	}
	if in.Type != nil {
		merged.Type = *in.Type
	}
	if in.TargetAmountMinor != nil {
		merged.TargetAmountMinor = *in.TargetAmountMinor
	}
	if in.Deadline != nil {
		merged.Deadline = in.Deadline
	}
	if in.LinkedAccountID != nil {
		merged.LinkedAccountID = in.LinkedAccountID
		merged.ManualCurrentAmountMinor = nil
	}
	if in.ManualCurrentAmountMinor != nil {
		merged.ManualCurrentAmountMinor = in.ManualCurrentAmountMinor
		merged.LinkedAccountID = nil
	}
	if in.Status != nil {
		merged.Status = *in.Status
	}
	if merged.Type == "global" {
		merged.LinkedAccountID = nil
		merged.ManualCurrentAmountMinor = nil
	}
	merged.Name = strings.TrimSpace(merged.Name)
	if err := validateGoal(merged); err != nil {
		return domain.Goal{}, err
	}
	if err := s.validateGoalAccount(ctx, userID, merged.LinkedAccountID); err != nil {
		return domain.Goal{}, err
	}
	deadline, err := parseGoalDeadline(merged.Deadline)
	if err != nil {
		return domain.Goal{}, err
	}
	tag, err := s.db.Exec(ctx, `UPDATE goals SET name=$3,type=$4,target_amount_minor=$5,deadline=$6,linked_account_id=$7,manual_current_amount_minor=$8,status=$9,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, merged.Name, merged.Type, merged.TargetAmountMinor, deadline, merged.LinkedAccountID, merged.ManualCurrentAmountMinor, merged.Status)
	if err != nil {
		return domain.Goal{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Goal{}, ErrNotFound
	}
	return s.GetGoal(ctx, userID, id, now)
}

func (s *Store) ArchiveGoal(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `UPDATE goals SET status='archived',updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

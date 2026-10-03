package store

import (
	"context"
	"errors"
	"math"
	"time"

	"finance-tracker/backend/internal/calculation"
	"finance-tracker/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type BudgetInput struct {
	CategoryID  string  `json:"category_id"`
	AmountMinor int64   `json:"amount_minor"`
	Period      string  `json:"period"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date"`
	IsActive    *bool   `json:"is_active"`
}

func validateBudgetInput(in BudgetInput) (time.Time, *time.Time, error) {
	if in.AmountMinor <= 0 {
		return time.Time{}, nil, ValidationError{"amount_minor must be greater than zero"}
	}
	if in.Period == "" {
		in.Period = "monthly"
	}
	if in.Period != "monthly" {
		return time.Time{}, nil, ValidationError{"period must be monthly"}
	}
	if _, err := uuid.Parse(in.CategoryID); err != nil {
		return time.Time{}, nil, ValidationError{"category_id must be a UUID"}
	}
	start, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return time.Time{}, nil, ValidationError{"start_date must use YYYY-MM-DD"}
	}
	var end *time.Time
	if in.EndDate != nil && *in.EndDate != "" {
		value, parseErr := time.Parse("2006-01-02", *in.EndDate)
		if parseErr != nil {
			return time.Time{}, nil, ValidationError{"end_date must use YYYY-MM-DD"}
		}
		if value.Before(start) {
			return time.Time{}, nil, ValidationError{"end_date must not be before start_date"}
		}
		end = &value
	}
	return start, end, nil
}

func (s *Store) budget(ctx context.Context, userID, id string, now time.Time) (domain.Budget, error) {
	var b domain.Budget
	err := s.db.QueryRow(ctx, `SELECT b.id,b.user_id,b.category_id,c.name,b.amount_minor,b.period,b.start_date,b.end_date,b.is_active,b.created_at,b.updated_at FROM budgets b JOIN categories c ON c.id=b.category_id WHERE b.user_id=$1 AND b.id=$2`, userID, id).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.CategoryName, &b.AmountMinor, &b.Period, &b.StartDate, &b.EndDate, &b.IsActive, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	if err = s.fillBudget(ctx, userID, &b, now); err != nil {
		return b, err
	}
	return b, nil
}
func (s *Store) fillBudget(ctx context.Context, userID string, b *domain.Budget, now time.Time) error {
	loc := now.Location()
	monthFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthTo := monthFrom.AddDate(0, 1, 0)
	activeFrom := monthFrom
	y, m, d := b.StartDate.Date()
	budgetStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	if budgetStart.After(activeFrom) {
		activeFrom = budgetStart
	}
	activeTo := monthTo
	if b.EndDate != nil {
		y, m, d := b.EndDate.Date()
		end := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
		if end.Before(activeTo) {
			activeTo = end
		}
	}
	if !activeFrom.Before(activeTo) {
		return nil
	}
	through := now
	if through.After(activeTo) {
		through = activeTo
	}
	if through.Before(activeFrom) {
		through = activeFrom
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount_minor),0) FROM transactions WHERE user_id=$1 AND category_id=$2 AND type='expense' AND occurred_at>=$3 AND occurred_at<$4`, userID, b.CategoryID, activeFrom, through).Scan(&b.SpentMinor); err != nil {
		return err
	}
	totalDays := int(activeTo.Sub(activeFrom).Hours() / 24)
	elapsed := int(math.Ceil(through.Sub(activeFrom).Hours() / 24))
	if elapsed < 1 && now.After(activeFrom) {
		elapsed = 1
	}
	b.RemainingMinor, b.UsagePercent, b.ProjectedSpendMinor = calculation.BudgetProgress(b.AmountMinor, b.SpentMinor, totalDays, elapsed)
	remaining := int(math.Ceil(activeTo.Sub(now).Hours() / 24))
	if remaining < 0 {
		remaining = 0
	}
	b.DaysRemaining = remaining
	return nil
}
func (s *Store) ListBudgets(ctx context.Context, userID string, now time.Time) ([]domain.Budget, error) {
	rows, err := s.db.Query(ctx, `SELECT b.id,b.user_id,b.category_id,c.name,b.amount_minor,b.period,b.start_date,b.end_date,b.is_active,b.created_at,b.updated_at FROM budgets b JOIN categories c ON c.id=b.category_id WHERE b.user_id=$1 ORDER BY b.is_active DESC,c.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Budget{}
	for rows.Next() {
		var b domain.Budget
		if err = rows.Scan(&b.ID, &b.UserID, &b.CategoryID, &b.CategoryName, &b.AmountMinor, &b.Period, &b.StartDate, &b.EndDate, &b.IsActive, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		if err = s.fillBudget(ctx, userID, &b, now); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}
func (s *Store) GetBudget(ctx context.Context, userID, id string, now time.Time) (domain.Budget, error) {
	return s.budget(ctx, userID, id, now)
}
func (s *Store) CreateBudget(ctx context.Context, userID string, in BudgetInput, now time.Time) (domain.Budget, error) {
	start, end, err := validateBudgetInput(in)
	if err != nil {
		return domain.Budget{}, err
	}
	var kind string
	if err = s.db.QueryRow(ctx, `SELECT type FROM categories WHERE id=$1 AND (user_id IS NULL OR user_id=$2)`, in.CategoryID, userID).Scan(&kind); errors.Is(err, pgx.ErrNoRows) {
		return domain.Budget{}, ValidationError{"expense category not found"}
	} else if err != nil {
		return domain.Budget{}, err
	}
	if kind != "expense" {
		return domain.Budget{}, ValidationError{"budget category must be expense"}
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	id := uuid.NewString()
	_, err = s.db.Exec(ctx, `INSERT INTO budgets(id,user_id,category_id,amount_minor,period,start_date,end_date,is_active) VALUES($1,$2,$3,$4,'monthly',$5,$6,$7)`, id, userID, in.CategoryID, in.AmountMinor, start, end, active)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Budget{}, ErrConflict
		}
		return domain.Budget{}, err
	}
	return s.budget(ctx, userID, id, now)
}
func (s *Store) UpdateBudget(ctx context.Context, userID, id string, in BudgetInput, now time.Time) (domain.Budget, error) {
	if _, err := s.budget(ctx, userID, id, now); err != nil {
		return domain.Budget{}, err
	}
	start, end, err := validateBudgetInput(in)
	if err != nil {
		return domain.Budget{}, err
	}
	var kind string
	if err = s.db.QueryRow(ctx, `SELECT type FROM categories WHERE id=$1 AND (user_id IS NULL OR user_id=$2)`, in.CategoryID, userID).Scan(&kind); err != nil {
		return domain.Budget{}, ValidationError{"expense category not found"}
	}
	if kind != "expense" {
		return domain.Budget{}, ValidationError{"budget category must be expense"}
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	_, err = s.db.Exec(ctx, `UPDATE budgets SET category_id=$3,amount_minor=$4,start_date=$5,end_date=$6,is_active=$7,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, in.CategoryID, in.AmountMinor, start, end, active)
	if err != nil {
		return domain.Budget{}, err
	}
	return s.budget(ctx, userID, id, now)
}
func (s *Store) DisableBudget(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `UPDATE budgets SET is_active=false,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

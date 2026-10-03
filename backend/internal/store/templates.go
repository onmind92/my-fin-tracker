package store

import (
	"context"
	"errors"
	"strings"

	"finance-tracker/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TemplateInput struct {
	Name            string  `json:"name"`
	TransactionType string  `json:"transaction_type"`
	AmountMinor     *int64  `json:"amount_minor"`
	AccountID       *string `json:"account_id"`
	CategoryID      *string `json:"category_id"`
	Description     *string `json:"description"`
}

const templateSelect = `SELECT tt.id,tt.user_id,tt.name,tt.transaction_type,tt.amount_minor,tt.account_id,a.name,tt.category_id,c.name,tt.description,tt.created_at,tt.updated_at FROM transaction_templates tt LEFT JOIN accounts a ON a.id=tt.account_id LEFT JOIN categories c ON c.id=tt.category_id WHERE tt.user_id=$1`

func scanTemplate(row pgx.Row) (domain.TransactionTemplate, error) {
	var item domain.TransactionTemplate
	err := row.Scan(&item.ID, &item.UserID, &item.Name, &item.TransactionType, &item.AmountMinor, &item.AccountID, &item.AccountName, &item.CategoryID, &item.CategoryName, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func validateTemplate(in TemplateInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return ValidationError{"name must not be empty"}
	}
	if in.TransactionType != "expense" && in.TransactionType != "income" {
		return ValidationError{"transaction_type must be expense or income"}
	}
	if in.AmountMinor != nil && *in.AmountMinor <= 0 {
		return ValidationError{"amount_minor must be greater than zero or null"}
	}
	return nil
}

func (s *Store) validateOptionalReferences(ctx context.Context, userID, kind string, accountID, categoryID *string, requireAccount bool) error {
	if requireAccount && accountID == nil {
		return ValidationError{"account_id is required"}
	}
	accountOwned, accountActive := accountID == nil, false
	if accountID != nil {
		err := s.db.QueryRow(ctx, `SELECT is_active FROM accounts WHERE id=$1 AND user_id=$2`, *accountID, userID).Scan(&accountActive)
		if errors.Is(err, pgx.ErrNoRows) {
			accountOwned = false
		} else if err != nil {
			return err
		} else {
			accountOwned = true
		}
	}
	categoryAvailable, categoryType := categoryID == nil, ""
	if categoryID != nil {
		err := s.db.QueryRow(ctx, `SELECT type FROM categories WHERE id=$1 AND (user_id IS NULL OR user_id=$2)`, *categoryID, userID).Scan(&categoryType)
		if errors.Is(err, pgx.ErrNoRows) {
			categoryAvailable = false
		} else if err != nil {
			return err
		} else {
			categoryAvailable = true
		}
	}
	return validateReferenceState(accountID != nil, accountOwned, accountActive, categoryID != nil, categoryAvailable, categoryType, kind)
}

func validateReferenceState(accountProvided, accountOwned, accountActive, categoryProvided, categoryAvailable bool, categoryType, transactionType string) error {
	if accountProvided && !accountOwned {
		return ValidationError{"account does not exist or is not owned by user"}
	}
	if accountProvided && !accountActive {
		return ValidationError{"account is inactive"}
	}
	if categoryProvided && !categoryAvailable {
		return ValidationError{"category does not exist or is not available to user"}
	}
	if categoryProvided && categoryType != transactionType {
		return ValidationError{"category type must match transaction type"}
	}
	return nil
}

func (s *Store) ListTemplates(ctx context.Context, userID string) ([]domain.TransactionTemplate, error) {
	rows, err := s.db.Query(ctx, templateSelect+` ORDER BY tt.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TransactionTemplate, 0)
	for rows.Next() {
		item, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) GetTemplate(ctx context.Context, userID, id string) (domain.TransactionTemplate, error) {
	return scanTemplate(s.db.QueryRow(ctx, templateSelect+` AND tt.id=$2`, userID, id))
}
func (s *Store) CreateTemplate(ctx context.Context, userID string, in TemplateInput) (domain.TransactionTemplate, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := validateTemplate(in); err != nil {
		return domain.TransactionTemplate{}, err
	}
	if err := s.validateOptionalReferences(ctx, userID, in.TransactionType, in.AccountID, in.CategoryID, false); err != nil {
		return domain.TransactionTemplate{}, err
	}
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO transaction_templates(id,user_id,name,transaction_type,amount_minor,account_id,category_id,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, userID, in.Name, in.TransactionType, in.AmountMinor, in.AccountID, in.CategoryID, in.Description)
	if err != nil {
		return domain.TransactionTemplate{}, err
	}
	return s.GetTemplate(ctx, userID, id)
}
func (s *Store) UpdateTemplate(ctx context.Context, userID, id string, in TemplateInput) (domain.TransactionTemplate, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := validateTemplate(in); err != nil {
		return domain.TransactionTemplate{}, err
	}
	if err := s.validateOptionalReferences(ctx, userID, in.TransactionType, in.AccountID, in.CategoryID, false); err != nil {
		return domain.TransactionTemplate{}, err
	}
	tag, err := s.db.Exec(ctx, `UPDATE transaction_templates SET name=$3,transaction_type=$4,amount_minor=$5,account_id=$6,category_id=$7,description=$8,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, in.Name, in.TransactionType, in.AmountMinor, in.AccountID, in.CategoryID, in.Description)
	if err != nil {
		return domain.TransactionTemplate{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.TransactionTemplate{}, ErrNotFound
	}
	return s.GetTemplate(ctx, userID, id)
}
func (s *Store) DeleteTemplate(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM transaction_templates WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

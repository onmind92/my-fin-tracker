package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ImportTransaction struct {
	SourceIndex int
	AccountID   string
	CategoryID  *string
	Type        string
	AmountMinor int64
	Description *string
	OccurredAt  time.Time
}

func (s *Store) ResolveImportCategory(ctx context.Context, userID, name, kind string) (*string, error) {
	if name == "" {
		return nil, nil
	}
	var id string
	err := s.db.QueryRow(ctx, `SELECT id FROM categories WHERE lower(name)=lower($1) AND type=$2 AND (user_id IS NULL OR user_id=$3) ORDER BY user_id NULLS LAST LIMIT 1`, name, kind, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ValidationError{"category not found: " + name}
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
func (s *Store) IsPossibleDuplicate(ctx context.Context, userID string, in ImportTransaction) (bool, error) {
	var exists bool
	description := ""
	if in.Description != nil {
		description = *in.Description
	}
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM transactions WHERE user_id=$1 AND account_id=$2 AND occurred_at=$3 AND amount_minor=$4 AND type=$5 AND COALESCE(description,'')=$6)`, userID, in.AccountID, in.OccurredAt, in.AmountMinor, in.Type, description).Scan(&exists)
	return exists, err
}
func (s *Store) ImportTransactions(ctx context.Context, userID string, items []ImportTransaction) (int, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	for _, item := range items {
		var active bool
		if err = tx.QueryRow(ctx, `SELECT is_active FROM accounts WHERE id=$1 AND user_id=$2`, item.AccountID, userID).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
			return 0, ValidationError{"import account is not owned by user"}
		} else if err != nil {
			return 0, err
		}
		if !active {
			return 0, ValidationError{"import account is inactive"}
		}
		if item.CategoryID != nil {
			var kind string
			if err = tx.QueryRow(ctx, `SELECT type FROM categories WHERE id=$1 AND (user_id IS NULL OR user_id=$2)`, *item.CategoryID, userID).Scan(&kind); err != nil || kind != item.Type {
				return 0, ValidationError{"import category is invalid"}
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO transactions(id,user_id,account_id,category_id,type,amount_minor,description,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.NewString(), userID, item.AccountID, item.CategoryID, item.Type, item.AmountMinor, item.Description, item.OccurredAt)
		if err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(items), nil
}

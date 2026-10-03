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

type TransferInput struct {
	FromAccountID string     `json:"from_account_id"`
	ToAccountID   string     `json:"to_account_id"`
	AmountMinor   int64      `json:"amount_minor"`
	Description   *string    `json:"description"`
	OccurredAt    *time.Time `json:"occurred_at"`
}

const transferSelect = `SELECT tr.id,tr.user_id,tr.from_account_id,fa.name,tr.to_account_id,ta.name,tr.amount_minor,tr.description,tr.occurred_at,tr.created_at,tr.updated_at FROM transfers tr JOIN accounts fa ON fa.id=tr.from_account_id JOIN accounts ta ON ta.id=tr.to_account_id WHERE tr.user_id=$1`

func scanTransfer(row pgx.Row) (domain.Transfer, error) {
	var item domain.Transfer
	err := row.Scan(&item.ID, &item.UserID, &item.FromAccountID, &item.FromAccountName, &item.ToAccountID, &item.ToAccountName, &item.AmountMinor, &item.Description, &item.OccurredAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (s *Store) validateTransfer(ctx context.Context, userID string, in TransferInput) error {
	if in.AmountMinor <= 0 {
		return ValidationError{"amount_minor must be greater than zero"}
	}
	if in.FromAccountID == in.ToAccountID {
		return ValidationError{"from_account_id and to_account_id must differ"}
	}
	if _, err := uuid.Parse(in.FromAccountID); err != nil {
		return ValidationError{"from_account_id must be a UUID"}
	}
	if _, err := uuid.Parse(in.ToAccountID); err != nil {
		return ValidationError{"to_account_id must be a UUID"}
	}
	rows, err := s.db.Query(ctx, `SELECT id,currency,is_active FROM accounts WHERE user_id=$1 AND id IN ($2,$3)`, userID, in.FromAccountID, in.ToAccountID)
	if err != nil {
		return err
	}
	defer rows.Close()
	currencies := map[string]string{}
	for rows.Next() {
		var id, currency string
		var active bool
		if err = rows.Scan(&id, &currency, &active); err != nil {
			return err
		}
		if !active {
			return ValidationError{"both accounts must be active"}
		}
		currencies[id] = currency
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(currencies) != 2 {
		return ValidationError{"accounts do not exist or are not owned by user"}
	}
	if currencies[in.FromAccountID] != currencies[in.ToAccountID] {
		return ValidationError{"account currencies must match"}
	}
	return nil
}

func (s *Store) ListTransfers(ctx context.Context, userID string) ([]domain.Transfer, error) {
	rows, err := s.db.Query(ctx, transferSelect+` ORDER BY tr.occurred_at DESC,tr.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Transfer{}
	for rows.Next() {
		item, scanErr := scanTransfer(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) GetTransfer(ctx context.Context, userID, id string) (domain.Transfer, error) {
	return scanTransfer(s.db.QueryRow(ctx, transferSelect+` AND tr.id=$2`, userID, id))
}
func (s *Store) CreateTransfer(ctx context.Context, userID string, in TransferInput) (domain.Transfer, error) {
	if err := s.validateTransfer(ctx, userID, in); err != nil {
		return domain.Transfer{}, err
	}
	when := time.Now().UTC()
	if in.OccurredAt != nil {
		when = *in.OccurredAt
	}
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		in.Description = &v
	}
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO transfers(id,user_id,from_account_id,to_account_id,amount_minor,description,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, userID, in.FromAccountID, in.ToAccountID, in.AmountMinor, in.Description, when)
	if err != nil {
		return domain.Transfer{}, err
	}
	return s.GetTransfer(ctx, userID, id)
}
func (s *Store) UpdateTransfer(ctx context.Context, userID, id string, in TransferInput) (domain.Transfer, error) {
	if _, err := s.GetTransfer(ctx, userID, id); err != nil {
		return domain.Transfer{}, err
	}
	if err := s.validateTransfer(ctx, userID, in); err != nil {
		return domain.Transfer{}, err
	}
	when := time.Now().UTC()
	if in.OccurredAt != nil {
		when = *in.OccurredAt
	}
	tag, err := s.db.Exec(ctx, `UPDATE transfers SET from_account_id=$3,to_account_id=$4,amount_minor=$5,description=$6,occurred_at=$7,updated_at=now() WHERE user_id=$1 AND id=$2`, userID, id, in.FromAccountID, in.ToAccountID, in.AmountMinor, in.Description, when)
	if err != nil {
		return domain.Transfer{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Transfer{}, ErrNotFound
	}
	return s.GetTransfer(ctx, userID, id)
}
func (s *Store) DeleteTransfer(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM transfers WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AccountHistory(ctx context.Context, userID, accountID, rangeName string, now time.Time) ([]domain.AccountBalancePoint, error) {
	account, err := s.GetAccount(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	_, ledger, err := s.loadLedger(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	points := historyDates(rangeName, now, []calculation.LedgerAccount{{ID: account.ID, OpeningBalanceMinor: account.OpeningBalanceMinor, CreatedAt: account.CreatedAt}})
	result := make([]domain.AccountBalancePoint, 0, len(points))
	for _, point := range points {
		value := calculation.NetWorthAt([]calculation.LedgerAccount{{ID: account.ID, OpeningBalanceMinor: account.OpeningBalanceMinor, CreatedAt: account.CreatedAt}}, ledger, point)
		result = append(result, domain.AccountBalancePoint{Date: point.Format("2006-01-02"), BalanceMinor: value})
	}
	return result, nil
}

package store

import (
	"context"
	"errors"

	"finance-tracker/backend/internal/exportdata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RestoreResult struct {
	Accounts     int `json:"accounts"`
	Transactions int `json:"transactions"`
	Transfers    int `json:"transfers"`
	Goals        int `json:"goals"`
	Templates    int `json:"templates"`
	Recurring    int `json:"recurring"`
	Budgets      int `json:"budgets"`
	Executions   int `json:"executions"`
}

func (s *Store) RestoreImported(ctx context.Context, userID, archiveID string) bool {
	var exists bool
	_ = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM restore_imports WHERE user_id=$1 AND archive_id=$2)`, userID, archiveID).Scan(&exists)
	return exists
}

func mapped(values map[string]string, old, kind string) (string, error) {
	value, ok := values[old]
	if !ok {
		return "", ValidationError{Message: "archive references unknown " + kind}
	}
	return value, nil
}

func (s *Store) RestoreArchive(ctx context.Context, userID, archiveID string, data exportdata.Data) (RestoreResult, error) {
	result := RestoreResult{}
	if _, err := uuid.Parse(archiveID); err != nil {
		return result, ValidationError{"archive_id must be a UUID"}
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO restore_imports(id,user_id,archive_id) VALUES($1,$2,$3)`, uuid.NewString(), userID, archiveID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return result, ErrConflict
		}
		return result, err
	}
	categoryMap := map[string]string{}
	for _, item := range data.Categories {
		var id string
		if item.UserID == nil {
			err = tx.QueryRow(ctx, `SELECT id FROM categories WHERE id=$1 OR (user_id IS NULL AND lower(name)=lower($2) AND type=$3) ORDER BY (id=$1) DESC LIMIT 1`, item.ID, item.Name, item.Type).Scan(&id)
		} else {
			err = tx.QueryRow(ctx, `SELECT id FROM categories WHERE user_id=$1 AND lower(name)=lower($2) AND type=$3`, userID, item.Name, item.Type).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				id = uuid.NewString()
				_, err = tx.Exec(ctx, `INSERT INTO categories(id,user_id,name,type,icon) VALUES($1,$2,$3,$4,$5)`, id, userID, item.Name, item.Type, item.Icon)
			}
		}
		if err != nil {
			return result, err
		}
		categoryMap[item.ID] = id
	}
	accountMap := map[string]string{}
	for _, item := range data.Accounts {
		id := uuid.NewString()
		_, err = tx.Exec(ctx, `INSERT INTO accounts(id,user_id,name,type,currency,opening_balance_minor,is_active) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, userID, item.Name, item.Type, item.Currency, item.OpeningBalanceMinor, item.IsActive)
		if err != nil {
			return result, err
		}
		accountMap[item.ID] = id
		result.Accounts++
	}
	recurringMap := map[string]string{}
	for _, item := range data.RecurringTransactions {
		accountID, mapErr := mapped(accountMap, item.AccountID, "recurring account")
		if mapErr != nil {
			return result, mapErr
		}
		var categoryID *string
		if item.CategoryID != nil {
			v, mapErr := mapped(categoryMap, *item.CategoryID, "recurring category")
			if mapErr != nil {
				return result, mapErr
			}
			categoryID = &v
		}
		id := uuid.NewString()
		_, err = tx.Exec(ctx, `INSERT INTO recurring_transactions(id,user_id,transaction_type,amount_minor,account_id,category_id,description,frequency,interval_value,start_date,next_run_at,end_date,local_time,is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, id, userID, item.TransactionType, item.AmountMinor, accountID, categoryID, item.Description, item.Frequency, item.Interval, item.StartDate, item.NextRunAt, item.EndDate, item.LocalTime, item.IsActive)
		if err != nil {
			return result, err
		}
		recurringMap[item.ID] = id
		result.Recurring++
	}
	transactionMap := map[string]string{}
	for _, item := range data.Transactions {
		accountID, mapErr := mapped(accountMap, item.AccountID, "transaction account")
		if mapErr != nil {
			return result, mapErr
		}
		var categoryID, recurringID *string
		if item.CategoryID != nil {
			v, mapErr := mapped(categoryMap, *item.CategoryID, "transaction category")
			if mapErr != nil {
				return result, mapErr
			}
			categoryID = &v
		}
		if item.RecurringTransactionID != nil {
			v, mapErr := mapped(recurringMap, *item.RecurringTransactionID, "transaction recurring schedule")
			if mapErr != nil {
				return result, mapErr
			}
			recurringID = &v
		}
		id := uuid.NewString()
		_, err = tx.Exec(ctx, `INSERT INTO transactions(id,user_id,account_id,category_id,type,amount_minor,description,occurred_at,recurring_transaction_id,scheduled_for) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, userID, accountID, categoryID, item.Type, item.AmountMinor, item.Description, item.OccurredAt, recurringID, item.ScheduledFor)
		if err != nil {
			return result, err
		}
		transactionMap[item.ID] = id
		result.Transactions++
	}
	for _, item := range data.Transfers {
		fromID, mapErr := mapped(accountMap, item.FromAccountID, "transfer source")
		if mapErr != nil {
			return result, mapErr
		}
		toID, mapErr := mapped(accountMap, item.ToAccountID, "transfer destination")
		if mapErr != nil {
			return result, mapErr
		}
		_, err = tx.Exec(ctx, `INSERT INTO transfers(id,user_id,from_account_id,to_account_id,amount_minor,description,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, uuid.NewString(), userID, fromID, toID, item.AmountMinor, item.Description, item.OccurredAt)
		if err != nil {
			return result, err
		}
		result.Transfers++
	}
	for _, item := range data.Goals {
		var linked *string
		if item.LinkedAccountID != nil {
			v, mapErr := mapped(accountMap, *item.LinkedAccountID, "goal account")
			if mapErr != nil {
				return result, mapErr
			}
			linked = &v
		}
		_, err = tx.Exec(ctx, `INSERT INTO goals(id,user_id,name,type,target_amount_minor,deadline,linked_account_id,manual_current_amount_minor,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuid.NewString(), userID, item.Name, item.Type, item.TargetAmountMinor, item.Deadline, linked, item.ManualCurrentAmountMinor, item.Status)
		if err != nil {
			return result, err
		}
		result.Goals++
	}
	for _, item := range data.TransactionTemplates {
		var accountID, categoryID *string
		if item.AccountID != nil {
			v, mapErr := mapped(accountMap, *item.AccountID, "template account")
			if mapErr != nil {
				return result, mapErr
			}
			accountID = &v
		}
		if item.CategoryID != nil {
			v, mapErr := mapped(categoryMap, *item.CategoryID, "template category")
			if mapErr != nil {
				return result, mapErr
			}
			categoryID = &v
		}
		_, err = tx.Exec(ctx, `INSERT INTO transaction_templates(id,user_id,name,transaction_type,amount_minor,account_id,category_id,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.NewString(), userID, item.Name, item.TransactionType, item.AmountMinor, accountID, categoryID, item.Description)
		if err != nil {
			return result, err
		}
		result.Templates++
	}
	for _, item := range data.Budgets {
		categoryID, mapErr := mapped(categoryMap, item.CategoryID, "budget category")
		if mapErr != nil {
			return result, mapErr
		}
		_, err = tx.Exec(ctx, `INSERT INTO budgets(id,user_id,category_id,amount_minor,period,start_date,end_date,is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.NewString(), userID, categoryID, item.AmountMinor, item.Period, item.StartDate, item.EndDate, item.IsActive)
		if err != nil {
			return result, err
		}
		result.Budgets++
	}
	for _, item := range data.RecurringExecutions {
		recurringID, mapErr := mapped(recurringMap, item.RecurringTransactionID, "execution schedule")
		if mapErr != nil {
			return result, mapErr
		}
		var transactionID *string
		if item.TransactionID != nil {
			if value, ok := transactionMap[*item.TransactionID]; ok {
				transactionID = &value
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO recurring_executions(id,recurring_transaction_id,user_id,scheduled_for,executed_at,transaction_id,status,error_message) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.NewString(), recurringID, userID, item.ScheduledFor, item.ExecutedAt, transactionID, item.Status, item.ErrorMessage)
		if err != nil {
			return result, err
		}
		result.Executions++
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

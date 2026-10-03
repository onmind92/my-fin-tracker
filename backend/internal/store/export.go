package store

import (
	"context"
	"time"

	"finance-tracker/backend/internal/domain"
	"finance-tracker/backend/internal/exportdata"
	"github.com/google/uuid"
)

func (s *Store) Export(ctx context.Context, user domain.User, now time.Time) (exportdata.Archive, error) {
	data := exportdata.Data{User: user}
	var err error
	if data.Accounts, err = s.ListAccounts(ctx, user.ID, false); err != nil {
		return exportdata.Archive{}, err
	}
	if data.Categories, err = s.ListCategories(ctx, user.ID); err != nil {
		return exportdata.Archive{}, err
	}
	if data.Transactions, err = s.ListTransactions(ctx, user.ID, TransactionFilter{Limit: 1_000_000}); err != nil {
		return exportdata.Archive{}, err
	}
	if data.Goals, err = s.ListGoals(ctx, user.ID, "", now); err != nil {
		return exportdata.Archive{}, err
	}
	if data.TransactionTemplates, err = s.ListTemplates(ctx, user.ID); err != nil {
		return exportdata.Archive{}, err
	}
	if data.RecurringTransactions, err = s.ListRecurring(ctx, user.ID); err != nil {
		return exportdata.Archive{}, err
	}
	if data.Transfers, err = s.ListTransfers(ctx, user.ID); err != nil {
		return exportdata.Archive{}, err
	}
	if data.Budgets, err = s.ListBudgets(ctx, user.ID, now); err != nil {
		return exportdata.Archive{}, err
	}
	if data.RecurringExecutions, err = s.ListAllRecurringExecutions(ctx, user.ID); err != nil {
		return exportdata.Archive{}, err
	}
	return exportdata.Archive{ExportVersion: 1, ArchiveID: uuid.NewString(), ExportedAt: now.UTC(), Data: data}, nil
}

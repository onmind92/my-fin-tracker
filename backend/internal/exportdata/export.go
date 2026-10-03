package exportdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"finance-tracker/backend/internal/domain"
)

type Data struct {
	User                  domain.User                   `json:"user"`
	Accounts              []domain.Account              `json:"accounts"`
	Categories            []domain.Category             `json:"categories"`
	Transactions          []domain.Transaction          `json:"transactions"`
	Goals                 []domain.Goal                 `json:"goals"`
	TransactionTemplates  []domain.TransactionTemplate  `json:"transaction_templates"`
	RecurringTransactions []domain.RecurringTransaction `json:"recurring_transactions"`
	Transfers             []domain.Transfer             `json:"transfers"`
	Budgets               []domain.Budget               `json:"budgets"`
	RecurringExecutions   []domain.RecurringExecution   `json:"recurring_executions"`
}
type Archive struct {
	ExportVersion int       `json:"export_version"`
	ArchiveID     string    `json:"archive_id"`
	ExportedAt    time.Time `json:"exported_at"`
	Data          Data      `json:"data"`
}

func WriteTransactionsCSV(writer io.Writer, items []domain.Transaction, location *time.Location) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"date", "type", "amount", "currency", "account", "category", "description"}); err != nil {
		return err
	}
	for _, item := range items {
		category, description := "", ""
		if item.CategoryName != nil {
			category = *item.CategoryName
		}
		if item.Description != nil {
			description = *item.Description
		}
		amount := fmt.Sprintf("%d.%02d", item.AmountMinor/100, item.AmountMinor%100)
		if err := csvWriter.Write([]string{item.OccurredAt.In(location).Format(time.RFC3339), item.Type, amount, "RUB", item.AccountName, category, description}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

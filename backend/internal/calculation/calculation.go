package calculation

import (
	"math"
	"time"

	"finance-tracker/backend/internal/domain"
)

type LedgerAccount struct {
	ID                  string
	OpeningBalanceMinor int64
	CreatedAt           time.Time
}

type LedgerTransaction struct {
	AccountID    string
	Type         string
	AmountMinor  int64
	OccurredAt   time.Time
	CategoryID   *string
	CategoryName string
}

func NetWorthAt(accounts []LedgerAccount, transactions []LedgerTransaction, at time.Time) int64 {
	balances := make(map[string]int64, len(accounts))
	for _, account := range accounts {
		if !account.CreatedAt.After(at) {
			balances[account.ID] = account.OpeningBalanceMinor
		}
	}
	for _, transaction := range transactions {
		if transaction.OccurredAt.After(at) {
			continue
		}
		if _, ok := balances[transaction.AccountID]; !ok {
			continue
		}
		if transaction.Type == "income" || transaction.Type == "transfer_in" {
			balances[transaction.AccountID] += transaction.AmountMinor
		} else if transaction.Type == "expense" || transaction.Type == "transfer_out" {
			balances[transaction.AccountID] -= transaction.AmountMinor
		}
	}
	var total int64
	for _, balance := range balances {
		total += balance
	}
	return total
}

func NetWorthHistory(accounts []LedgerAccount, transactions []LedgerTransaction, points []time.Time) []domain.NetWorthPoint {
	result := make([]domain.NetWorthPoint, 0, len(points))
	for _, point := range points {
		result = append(result, domain.NetWorthPoint{Date: point.Format("2006-01-02"), NetWorthMinor: NetWorthAt(accounts, transactions, point)})
	}
	return result
}

func MonthlyCashFlow(transactions []LedgerTransaction, from, to time.Time) []domain.CashFlowPoint {
	from = monthStart(from)
	toMonth := monthStart(to)
	if to.After(toMonth) {
		toMonth = toMonth.AddDate(0, 1, 0)
	}
	to = toMonth
	if !from.Before(to) {
		return []domain.CashFlowPoint{}
	}
	byMonth := map[string]*domain.CashFlowPoint{}
	for point := from; point.Before(to); point = point.AddDate(0, 1, 0) {
		key := point.Format("2006-01")
		byMonth[key] = &domain.CashFlowPoint{Period: key}
	}
	for _, transaction := range transactions {
		if transaction.OccurredAt.Before(from) || !transaction.OccurredAt.Before(to) {
			continue
		}
		item := byMonth[transaction.OccurredAt.In(from.Location()).Format("2006-01")]
		if item == nil {
			continue
		}
		if transaction.Type == "income" {
			item.IncomeMinor += transaction.AmountMinor
		}
		if transaction.Type == "expense" {
			item.ExpenseMinor += transaction.AmountMinor
		}
	}
	result := make([]domain.CashFlowPoint, 0, len(byMonth))
	for point := from; point.Before(to); point = point.AddDate(0, 1, 0) {
		item := byMonth[point.Format("2006-01")]
		item.CashFlowMinor = item.IncomeMinor - item.ExpenseMinor
		result = append(result, *item)
	}
	return result
}

func AverageCashFlow(points []domain.CashFlowPoint) int64 {
	if len(points) == 0 {
		return 0
	}
	var sum int64
	for _, point := range points {
		sum += point.CashFlowMinor
	}
	return sum / int64(len(points))
}

func BudgetProgress(limit, spent int64, totalDays, elapsedDays int) (int64, float64, int64) {
	remaining := limit - spent
	usage := float64(0)
	if limit > 0 {
		usage = math.Round(float64(spent)*10000/float64(limit)) / 100
	}
	projected := int64(0)
	if totalDays > 0 && elapsedDays > 0 {
		projected = int64(math.Round(float64(spent) * float64(totalDays) / float64(elapsedDays)))
	}
	return remaining, usage, projected
}

func ExpensesByCategory(transactions []LedgerTransaction, from, to time.Time) []domain.CategoryExpense {
	totals := map[string]*domain.CategoryExpense{}
	var total int64
	for _, transaction := range transactions {
		if transaction.Type != "expense" || transaction.OccurredAt.Before(from) || !transaction.OccurredAt.Before(to) {
			continue
		}
		key := "uncategorized"
		if transaction.CategoryID != nil {
			key = *transaction.CategoryID
		}
		item := totals[key]
		if item == nil {
			item = &domain.CategoryExpense{CategoryID: transaction.CategoryID, CategoryName: transaction.CategoryName}
			if item.CategoryName == "" {
				item.CategoryName = "Без категории"
			}
			totals[key] = item
		}
		item.AmountMinor += transaction.AmountMinor
		total += transaction.AmountMinor
	}
	result := make([]domain.CategoryExpense, 0, len(totals))
	for _, item := range totals {
		if total > 0 {
			item.SharePercent = math.Round(float64(item.AmountMinor)*10000/float64(total)) / 100
		}
		result = append(result, *item)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].AmountMinor > result[i].AmountMinor {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func Forecast(current, average int64, requested, actual int, now time.Time) domain.Forecast {
	result := domain.Forecast{
		CurrentNetWorthMinor: current, AverageMonthlyGrowthMinor: average,
		RequestedWindowMonths: requested, ActualWindowMonths: actual,
		ProjectionNotice: "Проекция при текущем среднем темпе, не гарантированный результат",
		Summary:          make([]domain.ForecastPoint, 0, 5), Forecast: make([]domain.ForecastPoint, 0, 24),
	}
	for month := 1; month <= 24; month++ {
		point := domain.ForecastPoint{MonthsFromNow: month, Date: now.AddDate(0, month, 0).Format("2006-01-02"), NetWorthMinor: current + average*int64(month)}
		result.Forecast = append(result.Forecast, point)
		if month == 1 || month == 3 || month == 6 || month == 12 || month == 24 {
			result.Summary = append(result.Summary, point)
		}
	}
	return result
}

type GoalInput struct {
	CurrentAmountMinor        int64
	TargetAmountMinor         int64
	Deadline                  *time.Time
	AverageMonthlyGrowthMinor int64
	Now                       time.Time
}

type GoalResult struct {
	ProgressPercent            float64
	RemainingAmountMinor       int64
	DaysRemaining              *int
	RequiredMonthlyAmountMinor *int64
	EstimatedCompletionDate    *time.Time
	ExpectedMonthsToComplete   *int
	Reached                    bool
}

func CalculateGoal(in GoalInput) GoalResult {
	result := GoalResult{}
	if in.TargetAmountMinor <= 0 {
		return result
	}
	rawProgress := float64(in.CurrentAmountMinor) * 100 / float64(in.TargetAmountMinor)
	result.ProgressPercent = math.Round(math.Max(0, math.Min(100, rawProgress))*100) / 100
	result.Reached = in.CurrentAmountMinor >= in.TargetAmountMinor
	result.RemainingAmountMinor = in.TargetAmountMinor - in.CurrentAmountMinor
	if result.RemainingAmountMinor < 0 {
		result.RemainingAmountMinor = 0
	}
	if in.Deadline != nil {
		nowDate := dateOnly(in.Now)
		deadline := dateOnly(*in.Deadline)
		days := int(deadline.Sub(nowDate).Hours() / 24)
		result.DaysRemaining = &days
		if result.Reached {
			zero := int64(0)
			result.RequiredMonthlyAmountMinor = &zero
		} else if days > 0 {
			months := int(math.Ceil(float64(days) / 30.4375))
			if months < 1 {
				months = 1
			}
			required := (result.RemainingAmountMinor + int64(months) - 1) / int64(months)
			result.RequiredMonthlyAmountMinor = &required
		}
	}
	if !result.Reached && in.AverageMonthlyGrowthMinor > 0 {
		months := int((result.RemainingAmountMinor + in.AverageMonthlyGrowthMinor - 1) / in.AverageMonthlyGrowthMinor)
		result.ExpectedMonthsToComplete = &months
		estimated := dateOnly(in.Now).AddDate(0, months, 0)
		result.EstimatedCompletionDate = &estimated
	} else if result.Reached {
		zero := 0
		result.ExpectedMonthsToComplete = &zero
		estimated := dateOnly(in.Now)
		result.EstimatedCompletionDate = &estimated
	}
	return result
}

func dateOnly(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func monthStart(value time.Time) time.Time {
	y, m, _ := value.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, value.Location())
}

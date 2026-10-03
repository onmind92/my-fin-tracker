package domain

import "time"

const DevUserID = "00000000-0000-0000-0000-000000000001"

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName *string   `json:"display_name,omitempty"`
	Timezone    string    `json:"timezone"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Account struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id,omitempty"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	Currency            string    `json:"currency"`
	OpeningBalanceMinor int64     `json:"opening_balance_minor"`
	CurrentBalanceMinor int64     `json:"current_balance_minor"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Category struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id,omitempty"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Icon      *string   `json:"icon,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Transaction struct {
	ID                     string     `json:"id"`
	UserID                 string     `json:"user_id,omitempty"`
	AccountID              string     `json:"account_id"`
	AccountName            string     `json:"account_name,omitempty"`
	CategoryID             *string    `json:"category_id,omitempty"`
	CategoryName           *string    `json:"category_name,omitempty"`
	CategoryIcon           *string    `json:"category_icon,omitempty"`
	Type                   string     `json:"type"`
	AmountMinor            int64      `json:"amount_minor"`
	Description            *string    `json:"description,omitempty"`
	OccurredAt             time.Time  `json:"occurred_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	RecurringTransactionID *string    `json:"recurring_transaction_id,omitempty"`
	ScheduledFor           *time.Time `json:"scheduled_for,omitempty"`
}

type TransactionTemplate struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id,omitempty"`
	Name            string    `json:"name"`
	TransactionType string    `json:"transaction_type"`
	AmountMinor     *int64    `json:"amount_minor,omitempty"`
	AccountID       *string   `json:"account_id,omitempty"`
	AccountName     *string   `json:"account_name,omitempty"`
	CategoryID      *string   `json:"category_id,omitempty"`
	CategoryName    *string   `json:"category_name,omitempty"`
	Description     *string   `json:"description,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RecurringTransaction struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id,omitempty"`
	TransactionType string     `json:"transaction_type"`
	AmountMinor     int64      `json:"amount_minor"`
	AccountID       string     `json:"account_id"`
	AccountName     string     `json:"account_name"`
	CategoryID      *string    `json:"category_id,omitempty"`
	CategoryName    *string    `json:"category_name,omitempty"`
	Description     *string    `json:"description,omitempty"`
	Frequency       string     `json:"frequency"`
	Interval        int        `json:"interval"`
	StartDate       time.Time  `json:"start_date"`
	NextRunAt       time.Time  `json:"next_run_at"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	LocalTime       string     `json:"local_time"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Transfer struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id,omitempty"`
	FromAccountID   string    `json:"from_account_id"`
	FromAccountName string    `json:"from_account_name"`
	ToAccountID     string    `json:"to_account_id"`
	ToAccountName   string    `json:"to_account_name"`
	AmountMinor     int64     `json:"amount_minor"`
	Description     *string   `json:"description,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Budget struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id,omitempty"`
	CategoryID          string     `json:"category_id"`
	CategoryName        string     `json:"category_name"`
	Period              string     `json:"period"`
	AmountMinor         int64      `json:"amount_minor"`
	StartDate           time.Time  `json:"start_date"`
	EndDate             *time.Time `json:"end_date,omitempty"`
	IsActive            bool       `json:"is_active"`
	SpentMinor          int64      `json:"spent_minor"`
	RemainingMinor      int64      `json:"remaining_minor"`
	UsagePercent        float64    `json:"usage_percent"`
	DaysRemaining       int        `json:"days_remaining"`
	ProjectedSpendMinor int64      `json:"projected_spend_minor"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type RecurringExecution struct {
	ID                     string    `json:"id"`
	RecurringTransactionID string    `json:"recurring_transaction_id"`
	UserID                 string    `json:"user_id,omitempty"`
	Status                 string    `json:"status"`
	ScheduledFor           time.Time `json:"scheduled_for"`
	ExecutedAt             time.Time `json:"executed_at"`
	TransactionID          *string   `json:"transaction_id,omitempty"`
	ErrorMessage           *string   `json:"error_message,omitempty"`
}

type AccountBalancePoint struct {
	Date         string `json:"date"`
	BalanceMinor int64  `json:"balance_minor"`
}

type MonthSummary struct {
	IncomeMinor   int64 `json:"income_minor"`
	ExpenseMinor  int64 `json:"expense_minor"`
	CashFlowMinor int64 `json:"cash_flow_minor"`
}

type Goal struct {
	ID                         string     `json:"id"`
	UserID                     string     `json:"user_id,omitempty"`
	Name                       string     `json:"name"`
	Type                       string     `json:"type"`
	TargetAmountMinor          int64      `json:"target_amount_minor"`
	Deadline                   *time.Time `json:"deadline,omitempty"`
	LinkedAccountID            *string    `json:"linked_account_id,omitempty"`
	LinkedAccountName          *string    `json:"linked_account_name,omitempty"`
	ManualCurrentAmountMinor   *int64     `json:"manual_current_amount_minor,omitempty"`
	Status                     string     `json:"status"`
	CurrentAmountMinor         int64      `json:"current_amount_minor"`
	ProgressPercent            float64    `json:"progress_percent"`
	RemainingAmountMinor       int64      `json:"remaining_amount_minor"`
	DaysRemaining              *int       `json:"days_remaining"`
	RequiredMonthlyAmountMinor *int64     `json:"required_monthly_amount_minor"`
	EstimatedCompletionDate    *time.Time `json:"estimated_completion_date,omitempty"`
	ExpectedMonthsToComplete   *int       `json:"expected_months_to_complete,omitempty"`
	Reached                    bool       `json:"reached"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

type NetWorthPoint struct {
	Date          string `json:"date"`
	NetWorthMinor int64  `json:"net_worth_minor"`
}

type CashFlowPoint struct {
	Period        string `json:"period"`
	IncomeMinor   int64  `json:"income_minor"`
	ExpenseMinor  int64  `json:"expense_minor"`
	CashFlowMinor int64  `json:"cash_flow_minor"`
}

type CategoryExpense struct {
	CategoryID   *string `json:"category_id,omitempty"`
	CategoryName string  `json:"category_name"`
	AmountMinor  int64   `json:"amount_minor"`
	SharePercent float64 `json:"share_percent"`
}

type ForecastPoint struct {
	MonthsFromNow int    `json:"months_from_now"`
	Date          string `json:"date"`
	NetWorthMinor int64  `json:"net_worth_minor"`
}

type Forecast struct {
	CurrentNetWorthMinor      int64           `json:"current_net_worth_minor"`
	AverageMonthlyGrowthMinor int64           `json:"average_monthly_growth_minor"`
	RequestedWindowMonths     int             `json:"requested_window_months"`
	ActualWindowMonths        int             `json:"actual_window_months"`
	ProjectionNotice          string          `json:"projection_notice"`
	Summary                   []ForecastPoint `json:"summary"`
	Forecast                  []ForecastPoint `json:"forecast"`
}

type NetWorthChange struct {
	AmountMinor int64    `json:"amount_minor"`
	Percent     *float64 `json:"percent"`
}

type ForecastSummary struct {
	AverageMonthlyGrowthMinor int64 `json:"average_monthly_growth_minor"`
	SixMonthsMinor            int64 `json:"six_months_minor"`
	TwelveMonthsMinor         int64 `json:"twelve_months_minor"`
}

type Dashboard struct {
	NetWorthMinor      int64           `json:"net_worth_minor"`
	Currency           string          `json:"currency"`
	Accounts           []Account       `json:"accounts"`
	Month              MonthSummary    `json:"month"`
	RecentTransactions []Transaction   `json:"recent_transactions"`
	NetWorthChange     NetWorthChange  `json:"net_worth_change"`
	ActiveGoals        []Goal          `json:"active_goals"`
	Forecast           ForecastSummary `json:"forecast"`
	BudgetPreview      []Budget        `json:"budget_preview"`
}

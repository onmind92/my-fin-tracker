export type AccountType = 'card' | 'cash' | 'savings' | 'investment' | 'other'
export type TransactionType = 'expense' | 'income'
export interface User { id:string;email:string;display_name?:string;timezone:string;created_at:string;updated_at:string }
export interface AuthResponse { user:User;csrf_token:string }

export interface Account {
  id: string; name: string; type: AccountType; currency: 'RUB'
  opening_balance_minor: number; current_balance_minor: number; is_active: boolean
  created_at: string; updated_at: string
}

export interface Category { id: string; name: string; type: TransactionType; icon?: string }

export interface Transaction {
  id: string; account_id: string; account_name: string; category_id?: string
  category_name?: string; category_icon?: string; type: TransactionType; amount_minor: number
  description?: string; occurred_at: string; created_at: string; updated_at: string
}

export interface Dashboard {
  net_worth_minor: number; currency: 'RUB'; accounts: Account[]
  month: { income_minor: number; expense_minor: number; cash_flow_minor: number }
  recent_transactions: Transaction[]
  net_worth_change: { amount_minor: number; percent?: number }
  active_goals: Goal[]
  forecast: { average_monthly_growth_minor: number; six_months_minor: number; twelve_months_minor: number }
  budget_preview: Budget[]
}

export interface TransactionInput {
  account_id: string; category_id?: string; type: TransactionType; amount_minor: number
  description?: string; occurred_at: string
}

export interface Transfer { id:string;from_account_id:string;from_account_name:string;to_account_id:string;to_account_name:string;amount_minor:number;description?:string;occurred_at:string;created_at:string;updated_at:string }
export interface TransferInput { from_account_id:string;to_account_id:string;amount_minor:number;description?:string;occurred_at:string }
export interface Budget { id:string;category_id:string;category_name:string;amount_minor:number;period:'monthly';start_date:string;end_date?:string;is_active:boolean;spent_minor:number;remaining_minor:number;usage_percent:number;days_remaining:number;projected_spend_minor:number }
export interface BudgetInput { category_id:string;amount_minor:number;period:'monthly';start_date:string;end_date?:string;is_active?:boolean }
export interface RecurringExecution { id:string;recurring_transaction_id:string;scheduled_for:string;executed_at:string;transaction_id?:string;status:'success'|'skipped'|'failed';error_message?:string }
export interface CSVMapping { date_column:string;amount_column:string;type_column:string;description_column:string;category_column:string;account_id:string }
export interface CSVPreviewRow {index:number;date:string;type:TransactionType;amount_minor:number;description:string;category:string;status:'new'|'possible_duplicate'|'invalid'}
export interface CSVPreview { token:string;headers:string[];rows:Array<string[]|CSVPreviewRow>;issues?:string[] }
export interface RestorePreview { token:string;archive_id:string;already_imported:boolean;counts:Record<string,number> }

export interface TransactionTemplate {
  id:string; name:string; transaction_type:TransactionType; amount_minor?:number
  account_id?:string; account_name?:string; category_id?:string; category_name?:string
  description?:string; created_at:string; updated_at:string
}
export interface TemplateInput { name:string; transaction_type:TransactionType; amount_minor?:number; account_id?:string; category_id?:string; description?:string }

export interface QuickParseResult {
  parsed:boolean; confidence:'high'|'partial'|'none'; unresolved:string[]
  transaction:{ type:TransactionType; amount_minor?:number; category_id?:string; category_name?:string; account_id?:string; account_name?:string; description?:string; occurred_at?:string }
}

export type RecurringFrequency='daily'|'weekly'|'monthly'|'yearly'
export interface RecurringTransaction {
  id:string; transaction_type:TransactionType; amount_minor:number; account_id:string; account_name:string
  category_id?:string; category_name?:string; description?:string; frequency:RecurringFrequency; interval:number
  start_date:string; next_run_at:string; end_date?:string; is_active:boolean; created_at:string; updated_at:string
  local_time:string
}
export interface RecurringInput { transaction_type:TransactionType; amount_minor:number; account_id:string; category_id?:string; description?:string; frequency:RecurringFrequency; interval:number; start_date:string; end_date?:string;local_time:string; is_active?:boolean }

export type GoalType = 'global' | 'current'
export type GoalStatus = 'active' | 'completed' | 'archived'

export interface Goal {
  id: string; name: string; type: GoalType; target_amount_minor: number; deadline?: string
  linked_account_id?: string; linked_account_name?: string; manual_current_amount_minor?: number
  status: GoalStatus; current_amount_minor: number; progress_percent: number; remaining_amount_minor: number
  days_remaining?: number; required_monthly_amount_minor?: number; estimated_completion_date?: string
  expected_months_to_complete?: number; reached: boolean; created_at: string; updated_at: string
}

export interface GoalInput {
  name: string; type: GoalType; target_amount_minor: number; deadline?: string
  linked_account_id?: string; manual_current_amount_minor?: number; status?: GoalStatus
}

export interface NetWorthPoint { date: string; net_worth_minor: number }
export interface CashFlowPoint { period: string; income_minor: number; expense_minor: number; cash_flow_minor: number }
export interface CategoryExpense { category_id?: string; category_name: string; amount_minor: number; share_percent: number }
export interface ForecastPoint { months_from_now: number; date: string; net_worth_minor: number }
export interface Forecast {
  current_net_worth_minor: number; average_monthly_growth_minor: number
  requested_window_months: number; actual_window_months: number; projection_notice: string
  summary: ForecastPoint[]; forecast: ForecastPoint[]
}

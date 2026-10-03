import type { Account, AccountType, AuthResponse, Budget, BudgetInput, CashFlowPoint, Category, CategoryExpense, CSVMapping, CSVPreview, Dashboard, Forecast, Goal, GoalInput, NetWorthPoint, QuickParseResult, RecurringExecution, RecurringInput, RecurringTransaction, RestorePreview, TemplateInput, Transaction, TransactionInput, TransactionTemplate, Transfer, TransferInput, User } from './types'

const BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'
let csrfToken=''
let unauthorizedHandler:()=>void=()=>{}
export const setCSRFToken=(value:string)=>{csrfToken=value}
export const setUnauthorizedHandler=(handler:()=>void)=>{unauthorizedHandler=handler}
async function upload<T>(path:string,file:File):Promise<T>{const response=await fetch(`${BASE}${path}`,{method:'POST',credentials:'include',headers:{'Content-Type':file.type||'application/octet-stream','X-CSRF-Token':csrfToken},body:file});if(response.status===401)unauthorizedHandler();if(!response.ok){const payload=await response.json().catch(()=>({error:'Не удалось загрузить файл'}));throw new Error(payload.error)};return response.json() as Promise<T>}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${BASE}${path}`, {
    ...options,
    credentials:'include',
    headers: { 'Content-Type': 'application/json',...(options?.method&&options.method!=='GET'&&csrfToken?{'X-CSRF-Token':csrfToken}:{}), ...options?.headers },
  }).catch(()=>{throw new Error('Нет соединения. Изменения не сохранены.')})
  if(response.status===401){unauthorizedHandler()}
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: 'Не удалось выполнить запрос' }))
    throw new Error(payload.error || `HTTP ${response.status}`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  me:()=>request<AuthResponse>('/api/v1/auth/me'),
  register:(body:{email:string;password:string;display_name?:string;timezone:string})=>request<AuthResponse>('/api/v1/auth/register',{method:'POST',body:JSON.stringify(body)}),
  login:(body:{email:string;password:string})=>request<AuthResponse>('/api/v1/auth/login',{method:'POST',body:JSON.stringify(body)}),
  logout:()=>request<void>('/api/v1/auth/logout',{method:'POST'}),
  updateMe:(body:{display_name:string;timezone:string})=>request<User>('/api/v1/auth/me',{method:'PATCH',body:JSON.stringify(body)}),
  dashboard: () => request<Dashboard>('/api/v1/dashboard'),
  accounts: () => request<Account[]>('/api/v1/accounts'),
  categories: () => request<Category[]>('/api/v1/categories'),
  transactions: (query = '') => request<Transaction[]>(`/api/v1/transactions${query}`),
  createAccount: (body: { name: string; type: AccountType; currency: 'RUB'; opening_balance_minor: number }) =>
    request<Account>('/api/v1/accounts', { method: 'POST', body: JSON.stringify(body) }),
  createTransaction: (body: TransactionInput) =>
    request<Transaction>('/api/v1/transactions', { method: 'POST', body: JSON.stringify(body) }),
  updateTransaction: (id:string, body:TransactionInput) => request<Transaction>(`/api/v1/transactions/${id}`, { method:'PATCH', body:JSON.stringify(body) }),
  deleteTransaction: (id: string) => request<void>(`/api/v1/transactions/${id}`, { method: 'DELETE' }),
  transfers:()=>request<Transfer[]>('/api/v1/transfers'),
  createTransfer:(body:TransferInput)=>request<Transfer>('/api/v1/transfers',{method:'POST',body:JSON.stringify(body)}),
  deleteTransfer:(id:string)=>request<void>(`/api/v1/transfers/${id}`,{method:'DELETE'}),
  budgets:()=>request<Budget[]>('/api/v1/budgets'),
  createBudget:(body:BudgetInput)=>request<Budget>('/api/v1/budgets',{method:'POST',body:JSON.stringify(body)}),
  updateBudget:(id:string,body:BudgetInput)=>request<Budget>(`/api/v1/budgets/${id}`,{method:'PATCH',body:JSON.stringify(body)}),
  deleteBudget:(id:string)=>request<void>(`/api/v1/budgets/${id}`,{method:'DELETE'}),
  goals: (status = '') => request<Goal[]>(`/api/v1/goals${status ? `?status=${status}` : ''}`),
  createGoal: (body: GoalInput) => request<Goal>('/api/v1/goals', { method: 'POST', body: JSON.stringify(body) }),
  updateGoal: (id: string, body: Partial<GoalInput>) => request<Goal>(`/api/v1/goals/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  archiveGoal: (id: string) => request<void>(`/api/v1/goals/${id}`, { method: 'DELETE' }),
  netWorth: (range = '6m') => request<NetWorthPoint[]>(`/api/v1/analytics/net-worth?range=${range}`),
  cashFlow: (range = '6m') => request<CashFlowPoint[]>(`/api/v1/analytics/cash-flow?range=${range}`),
  expensesByCategory: (range = '6m') => request<CategoryExpense[]>(`/api/v1/analytics/expenses-by-category?range=${range}`),
  forecast: (window = 6) => request<Forecast>(`/api/v1/analytics/forecast?average_window_months=${window}`),
  templates: () => request<TransactionTemplate[]>('/api/v1/transaction-templates'),
  createTemplate: (body:TemplateInput) => request<TransactionTemplate>('/api/v1/transaction-templates',{method:'POST',body:JSON.stringify(body)}),
  updateTemplate: (id:string,body:TemplateInput) => request<TransactionTemplate>(`/api/v1/transaction-templates/${id}`,{method:'PATCH',body:JSON.stringify(body)}),
  deleteTemplate: (id:string) => request<void>(`/api/v1/transaction-templates/${id}`,{method:'DELETE'}),
  parseQuickEntry: (text:string) => request<QuickParseResult>('/api/v1/quick-entry/parse',{method:'POST',body:JSON.stringify({text})}),
  recurring: () => request<RecurringTransaction[]>('/api/v1/recurring-transactions'),
  createRecurring: (body:RecurringInput) => request<RecurringTransaction>('/api/v1/recurring-transactions',{method:'POST',body:JSON.stringify(body)}),
  updateRecurring: (id:string,body:RecurringInput) => request<RecurringTransaction>(`/api/v1/recurring-transactions/${id}`,{method:'PATCH',body:JSON.stringify(body)}),
  disableRecurring: (id:string) => request<void>(`/api/v1/recurring-transactions/${id}`,{method:'DELETE'}),
  recurringExecutions:(id:string)=>request<RecurringExecution[]>(`/api/v1/recurring-transactions/${id}/executions`),
  csvUpload:(file:File)=>upload<CSVPreview>('/api/v1/import/csv/preview',file),
  csvAnalyze:(token:string,mapping:CSVMapping)=>request<CSVPreview>('/api/v1/import/csv/preview',{method:'POST',body:JSON.stringify({token,mapping})}),
  csvConfirm:(token:string,mapping:CSVMapping,row_indexes:number[])=>request<{imported:number}>('/api/v1/import/csv/confirm',{method:'POST',body:JSON.stringify({token,mapping,row_indexes})}),
  restoreUpload:(file:File)=>upload<RestorePreview>('/api/v1/restore/json/preview',file),
  restoreConfirm:(token:string)=>request<Record<string,number>>('/api/v1/restore/json/confirm',{method:'POST',body:JSON.stringify({token})}),
  downloadExport:async(path:'json'|'transactions.csv')=>{const response=await fetch(`${BASE}/api/v1/export/${path}`,{credentials:'include'});if(response.status===401){unauthorizedHandler();throw new Error('Сессия истекла')};if(!response.ok)throw new Error('Не удалось подготовить экспорт');return response.blob()},
}

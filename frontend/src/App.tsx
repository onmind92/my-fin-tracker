import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowDownLeft, ArrowRightLeft, ArrowUpRight, BarChart3, CalendarClock, CreditCard, Flag, Landmark, LayoutDashboard, List, PiggyBank, Plus, RefreshCw, Settings, TrendingUp, Upload, WalletCards, WifiOff, X } from 'lucide-react'
import { api,setCSRFToken,setUnauthorizedHandler } from './api/client'
import type { Account, Budget, BudgetInput, Category, Dashboard, Goal, GoalInput, NetWorthPoint, RecurringInput, RecurringTransaction, TemplateInput, Transaction, TransactionInput, TransactionTemplate, TransactionType, Transfer, TransferInput, User } from './api/types'
import { AccountModal } from './components/AccountModal'
import { TransactionList } from './components/TransactionList'
import { TransactionModal } from './components/TransactionModal'
import { GoalModal } from './components/GoalModal'
import { GoalCard } from './components/GoalCard'
import { RecurringModal } from './components/RecurringModal'
import { AuthScreen } from './components/AuthScreen'
import { TransferModal } from './components/TransferModal'
import { accountType, money } from './lib/format'

const AnalyticsPage=lazy(()=>import('./pages/AnalyticsPage').then((module)=>({default:module.AnalyticsPage})))
const GoalsPage=lazy(()=>import('./pages/GoalsPage').then((module)=>({default:module.GoalsPage})))
const RecurringPage=lazy(()=>import('./pages/RecurringPage'))
const SettingsPage=lazy(()=>import('./pages/SettingsPage'))
const BudgetsPage=lazy(()=>import('./pages/BudgetsPage'))
const DataPage=lazy(()=>import('./pages/DataPage'))
const NetWorthChart=lazy(()=>import('./components/NetWorthChart').then((module)=>({default:module.NetWorthChart})))

type Page = 'dashboard' | 'goals' | 'analytics' | 'recurring' | 'accounts' | 'transactions'|'budgets'|'data'|'settings'

export default function App() {
  const [user,setUser]=useState<User|null>(null)
  const [authLoading,setAuthLoading]=useState(true)
  const [online,setOnline]=useState(navigator.onLine)
  const [page, setPage] = useState<Page>('dashboard')
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [goals, setGoals] = useState<Goal[]>([])
  const [netWorthHistory, setNetWorthHistory] = useState<NetWorthPoint[]>([])
  const [templates,setTemplates]=useState<TransactionTemplate[]>([])
  const [recurring,setRecurring]=useState<RecurringTransaction[]>([])
  const [transfers,setTransfers]=useState<Transfer[]>([])
  const [budgets,setBudgets]=useState<Budget[]>([])
  const [chartRange, setChartRange] = useState('6m')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [transactionOpen, setTransactionOpen] = useState(false)
  const [accountOpen, setAccountOpen] = useState(false)
  const [goalOpen, setGoalOpen] = useState(false)
  const [editingGoal, setEditingGoal] = useState<Goal | undefined>()
  const [editingTransaction,setEditingTransaction]=useState<Transaction|undefined>()
  const [initialTransactionType,setInitialTransactionType]=useState<TransactionType>('expense')
  const [recurringOpen,setRecurringOpen]=useState(false)
  const [transferOpen,setTransferOpen]=useState(false)
  const [editingRecurring,setEditingRecurring]=useState<RecurringTransaction|undefined>()
  const [toast,setToast]=useState('')

  useEffect(()=>{const unauthenticated=()=>{setUser(null);setCSRFToken('');history.replaceState({},'','/login')};setUnauthorizedHandler(unauthenticated);void api.me().then((response)=>{setCSRFToken(response.csrf_token);setUser(response.user);if(window.location.pathname==='/login'||window.location.pathname==='/register')history.replaceState({},'','/')}).catch(()=>setUser(null)).finally(()=>setAuthLoading(false));const onlineHandler=()=>setOnline(true);const offlineHandler=()=>setOnline(false);window.addEventListener('online',onlineHandler);window.addEventListener('offline',offlineHandler);return()=>{window.removeEventListener('online',onlineHandler);window.removeEventListener('offline',offlineHandler)}},[])

  const notify=(message:string)=>{setToast(message);window.setTimeout(()=>setToast(''),2600)}
  const recentCategories=useMemo(()=>{const seen=new Set<string>();const result:Category[]=[];for(const transaction of transactions){if(!transaction.category_id||seen.has(transaction.category_id))continue;const category=categories.find((item)=>item.id===transaction.category_id);if(category){seen.add(category.id);result.push(category)}}return result},[transactions,categories])

  const load = useCallback(async () => {
    setError('')
    try {
      const [nextDashboard, nextAccounts, nextCategories, nextTransactions, nextGoals, nextHistory,nextTemplates,nextRecurring,nextTransfers,nextBudgets] = await Promise.all([api.dashboard(), api.accounts(), api.categories(), api.transactions('?limit=100'), api.goals(), api.netWorth(chartRange),api.templates(),api.recurring(),api.transfers(),api.budgets()])
      setDashboard(nextDashboard); setAccounts(nextAccounts); setCategories(nextCategories); setTransactions(nextTransactions); setGoals(nextGoals); setNetWorthHistory(nextHistory);setTemplates(nextTemplates);setRecurring(nextRecurring);setTransfers(nextTransfers);setBudgets(nextBudgets)
    } catch (err) { setError(err instanceof Error ? err.message : 'Не удалось загрузить данные') }
    finally { setLoading(false) }
  }, [chartRange])

  // Initial API synchronization; subsequent refreshes run after mutations.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { if(user)void load() }, [load,user])

  useEffect(()=>{const handler=(event:KeyboardEvent)=>{const target=event.target as HTMLElement|null;if(target&&(target.tagName==='INPUT'||target.tagName==='TEXTAREA'||target.tagName==='SELECT'||target.isContentEditable))return;const key=event.key.toLowerCase();let type:TransactionType|null=null;if(key==='n'||((event.ctrlKey||event.metaKey)&&key==='k')){event.preventDefault();type='expense'}else if(key==='e'){type='expense'}else if(key==='i'){type='income'}if(type){setInitialTransactionType(type);setEditingTransaction(undefined);setTransactionOpen(true)}};window.addEventListener('keydown',handler);return()=>window.removeEventListener('keydown',handler)},[])

  function openQuick(type:TransactionType){setInitialTransactionType(type);setEditingTransaction(undefined);setTransactionOpen(true)}
  async function saveTransaction(input: TransactionInput) { if(editingTransaction)await api.updateTransaction(editingTransaction.id,input);else await api.createTransaction(input);setTransactionOpen(false);setEditingTransaction(undefined);notify(editingTransaction?'Операция обновлена':'Операция добавлена');await load() }
  async function addAccount(input: Parameters<typeof api.createAccount>[0]) { await api.createAccount(input); setAccountOpen(false);notify('Счёт создан'); await load() }
  async function removeTransaction(id: string) { if (!window.confirm('Удалить эту операцию? Баланс будет пересчитан.')) return; await api.deleteTransaction(id);setTransactionOpen(false);setEditingTransaction(undefined);notify('Операция удалена'); await load() }
  async function saveTransfer(input:TransferInput){await api.createTransfer(input);setTransferOpen(false);notify('Перевод выполнен');await load()}
  async function removeTransfer(id:string){if(!window.confirm('Удалить перевод? Оба баланса будут пересчитаны.'))return;await api.deleteTransfer(id);await load()}
  async function createBudget(input:BudgetInput){await api.createBudget(input);notify('Бюджет создан');await load()}
  async function deleteBudget(id:string){await api.deleteBudget(id);notify('Бюджет архивирован');await load()}
  async function createTemplate(input:TemplateInput){const item=await api.createTemplate(input);setTemplates((current)=>[item,...current]);notify('Шаблон сохранён')}
  async function saveGoal(input: GoalInput) { if (editingGoal) await api.updateGoal(editingGoal.id, input); else await api.createGoal(input); setGoalOpen(false); setEditingGoal(undefined);notify(editingGoal?'Цель обновлена':'Цель создана'); await load() }
  async function archiveGoal(goal: Goal) { if (!window.confirm(`Архивировать цель «${goal.name}»?`)) return; await api.archiveGoal(goal.id);notify('Цель архивирована'); await load() }
  function editGoal(goal: Goal) { setEditingGoal(goal); setGoalOpen(true) }
  function createGoal() { setEditingGoal(undefined); setGoalOpen(true) }
  function changeChartRange(value:string){setChartRange(value)}
  function editTransaction(item:Transaction){setEditingTransaction(item);setTransactionOpen(true)}
  function createRecurring(){setEditingRecurring(undefined);setRecurringOpen(true)}
  function editRecurring(item:RecurringTransaction){setEditingRecurring(item);setRecurringOpen(true)}
  async function saveRecurring(input:RecurringInput){if(editingRecurring)await api.updateRecurring(editingRecurring.id,input);else await api.createRecurring(input);setRecurringOpen(false);setEditingRecurring(undefined);notify(editingRecurring?'Расписание обновлено':'Регулярная операция создана');await load()}
  async function toggleRecurring(item:RecurringTransaction){if(item.is_active){await api.disableRecurring(item.id)}else{const input:RecurringInput={transaction_type:item.transaction_type,amount_minor:item.amount_minor,account_id:item.account_id,category_id:item.category_id,description:item.description,frequency:item.frequency,interval:item.interval,start_date:item.start_date.slice(0,10),local_time:item.local_time,end_date:item.end_date?.slice(0,10),is_active:true};await api.updateRecurring(item.id,input)}notify(item.is_active?'Расписание приостановлено':'Расписание возобновлено');await load()}
  async function logout(){await api.logout();setCSRFToken('');setUser(null);history.replaceState({},'','/login')}

  const titles: Record<Page, [string, string]> = {
    dashboard: ['Обзор', 'Ваши финансы сегодня'], goals: ['Цели', 'Двигайтесь к важному в понятном темпе'], analytics: ['Аналитика', 'История и прозрачная проекция капитала'], recurring:['Регулярные','Автоматические операции по расписанию'], accounts: ['Счета', 'Все ваши деньги в одном месте'], transactions: ['Операции', 'Операции и переводы между счетами'],budgets:['Бюджеты','Месячные лимиты по категориям'],data:['Импорт','CSV и восстановление backup'],settings:['Настройки','Аккаунт, timezone и экспорт данных'],
  }

  if(authLoading)return <div className="grid min-h-[100dvh] place-items-center bg-canvas"><div className="h-10 w-10 animate-spin rounded-full border-4 border-emerald-100 border-t-emerald-700"/></div>
  if(!user&&!online)return <main className="grid min-h-[100dvh] place-items-center bg-canvas px-5 text-center"><div><div className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-amber-100 text-amber-700"><WifiOff/></div><h1 className="mt-4 text-xl font-bold">Нет соединения</h1><p className="mt-2 max-w-sm text-sm text-slate-500">Статические файлы доступны offline, но для безопасной проверки сессии и финансовых данных требуется сеть.</p></div></main>
  if(!user)return <AuthScreen onAuthenticated={(nextUser)=>{setUser(nextUser);setAuthLoading(false)}}/>

  return <div className="min-h-screen lg:grid lg:grid-cols-[240px_1fr]">
    <aside className="safe-bottom fixed bottom-0 z-40 flex w-full border-t border-slate-200 bg-white px-2 py-2 lg:sticky lg:top-0 lg:h-screen lg:flex-col lg:border-r lg:border-t-0 lg:px-5 lg:py-7">
      <div className="mb-9 hidden items-center gap-3 px-2 lg:flex"><div className="grid h-10 w-10 place-items-center rounded-xl bg-forest text-white"><Landmark size={20}/></div><div><p className="font-bold tracking-tight">Финансы</p><p className="text-[11px] text-slate-400">личный трекер</p></div></div>
      <nav className="flex w-full justify-start gap-1 overflow-x-auto lg:flex-col lg:overflow-visible">
        <NavButton active={page === 'dashboard'} onClick={() => setPage('dashboard')} icon={<LayoutDashboard size={19}/>} label="Обзор" />
        <NavButton active={page === 'goals'} onClick={() => setPage('goals')} icon={<Flag size={19}/>} label="Цели" />
        <NavButton active={page === 'analytics'} onClick={() => setPage('analytics')} icon={<BarChart3 size={19}/>} label="Аналитика" />
        <NavButton active={page === 'recurring'} onClick={() => setPage('recurring')} icon={<CalendarClock size={19}/>} label="Регулярные" />
        <NavButton active={page === 'accounts'} onClick={() => setPage('accounts')} icon={<WalletCards size={19}/>} label="Счета" />
        <NavButton active={page === 'transactions'} onClick={() => setPage('transactions')} icon={<List size={19}/>} label="Операции" />
        <NavButton active={page === 'budgets'} onClick={() => setPage('budgets')} icon={<PiggyBank size={19}/>} label="Бюджеты" />
        <NavButton active={page === 'data'} onClick={() => setPage('data')} icon={<Upload size={19}/>} label="Импорт" />
        <NavButton active={page === 'settings'} onClick={() => setPage('settings')} icon={<Settings size={19}/>} label="Ещё" />
      </nav>
      <div className="mt-auto hidden rounded-2xl bg-mint p-4 lg:block"><p className="text-xs font-semibold text-emerald-900">Быстрый ввод</p><p className="mt-1 text-xs leading-relaxed text-emerald-800/70"><kbd className="rounded bg-white/70 px-1.5 py-0.5">N</kbd> или <kbd className="rounded bg-white/70 px-1.5 py-0.5">Ctrl K</kbd> · E расход · I доход</p></div>
    </aside>
    <main className="min-w-0 px-4 pb-28 pt-6 sm:px-7 lg:px-10 lg:pb-12 lg:pt-9 xl:px-14">
      <div className="mx-auto max-w-6xl">
        <header className="mb-7 flex items-center justify-between gap-4"><div><p className="text-xs font-semibold uppercase tracking-[0.16em] text-emerald-700">Personal finance</p><h1 className="mt-1 text-2xl font-bold tracking-tight sm:text-3xl">{titles[page][0]}</h1><p className="mt-1 text-sm text-slate-500">{titles[page][1]}</p></div>{!['analytics','settings','budgets','data'].includes(page)&&<button onClick={() => page === 'accounts' ? setAccountOpen(true) : page==='goals'?createGoal():page==='recurring'?createRecurring():openQuick('expense')} className="flex shrink-0 items-center gap-2 rounded-xl bg-forest px-3.5 py-3 text-sm font-semibold text-white shadow-sm transition hover:bg-emerald-900 sm:px-4"><Plus size={18}/><span className="hidden sm:inline">{page === 'accounts' ? 'Новый счёт' : page==='goals'?'Новая цель':page==='recurring'?'Новое расписание':'Добавить операцию'}</span></button>}</header>
        {!online&&<div className="mb-5 flex items-center gap-2 rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800"><WifiOff size={17}/>Нет сети. Просмотр доступен частично, изменения не будут сохранены.</div>}
        {error && <div className="mb-6 flex items-center justify-between rounded-2xl border border-rose-100 bg-rose-50 px-4 py-3 text-sm text-rose-700"><span>{error}</span><button onClick={() => void load()} className="flex items-center gap-1 font-semibold"><RefreshCw size={15}/> Повторить</button></div>}
        {loading ? <Loading /> : <Suspense fallback={<PageSkeleton/>}>{page === 'dashboard' ? <DashboardPage data={dashboard} history={netWorthHistory} range={chartRange} onRangeChange={changeChartRange} onAddAccount={() => setAccountOpen(true)} onOpenAnalytics={()=>setPage('analytics')} /> : page==='goals'?<GoalsPage goals={goals} onCreate={createGoal} onEdit={editGoal} onArchive={archiveGoal}/>:page==='analytics'?<AnalyticsPage/>:page==='recurring'?<RecurringPage items={recurring} onCreate={createRecurring} onEdit={editRecurring} onToggle={toggleRecurring}/>:page==='budgets'?<BudgetsPage items={budgets} categories={categories} onCreate={createBudget} onDelete={deleteBudget}/>:page==='data'?<DataPage accounts={accounts} notify={notify} onChanged={load}/>:page==='settings'?<SettingsPage user={user} onUpdated={setUser} onLogout={logout} notify={notify} onOpenData={()=>setPage('data')}/>: page === 'accounts' ? <AccountsPage accounts={accounts} onAdd={() => setAccountOpen(true)} /> : <TransactionsPage items={transactions} transfers={transfers} onDelete={removeTransaction} onEdit={editTransaction} onAddTransfer={()=>setTransferOpen(true)} onDeleteTransfer={removeTransfer} />}</Suspense>}
      </div>
    </main>
    {transactionOpen && <TransactionModal accounts={accounts} categories={categories} templates={templates} recentCategories={recentCategories} transaction={editingTransaction} initialType={initialTransactionType} onClose={() => {setTransactionOpen(false);setEditingTransaction(undefined)}} onSubmit={saveTransaction} onTemplateCreated={createTemplate} onDelete={editingTransaction?()=>removeTransaction(editingTransaction.id):undefined} onTransferRequested={()=>{setTransactionOpen(false);setTransferOpen(true)}} />}
    {accountOpen && <AccountModal onClose={() => setAccountOpen(false)} onSubmit={addAccount} />}
    {goalOpen && <GoalModal accounts={accounts} goal={editingGoal} onClose={() => {setGoalOpen(false);setEditingGoal(undefined)}} onSubmit={saveGoal}/>} 
    {recurringOpen&&<RecurringModal accounts={accounts} categories={categories} item={editingRecurring} onClose={()=>{setRecurringOpen(false);setEditingRecurring(undefined)}} onSubmit={saveRecurring}/>} 
    {transferOpen&&<TransferModal accounts={accounts} onClose={()=>setTransferOpen(false)} onSubmit={saveTransfer}/>} 
    {!transactionOpen&&<button onClick={()=>openQuick('expense')} className="safe-fab fixed right-5 z-30 grid h-14 w-14 place-items-center rounded-full bg-forest text-white shadow-xl lg:hidden" aria-label="Добавить операцию"><Plus size={24}/></button>}
    {toast&&<div className="fixed bottom-24 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-3 rounded-xl bg-ink px-4 py-3 text-sm font-medium text-white shadow-xl lg:bottom-6">{toast}<button onClick={()=>setToast('')}><X size={15}/></button></div>}
  </div>
}

function NavButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: React.ReactNode; label: string }) {
  return <button onClick={onClick} className={`flex min-w-[58px] flex-1 flex-col items-center gap-1 rounded-xl px-2 py-2 text-[10px] font-medium transition lg:min-w-0 lg:flex-none lg:flex-row lg:gap-3 lg:px-3 lg:py-3 lg:text-sm ${active ? 'bg-mint text-forest' : 'text-slate-500 hover:bg-slate-50 hover:text-ink'}`}>{icon}<span>{label}</span></button>
}

function Loading() { return <div className="grid animate-pulse gap-4 sm:grid-cols-2 xl:grid-cols-4">{[1,2,3,4].map((i) => <div key={i} className="h-32 rounded-2xl bg-slate-200/70" />)}</div> }
function PageSkeleton(){return <div className="space-y-4 animate-pulse"><div className="h-64 rounded-2xl bg-slate-200/70"/><div className="grid gap-4 sm:grid-cols-2"><div className="h-48 rounded-2xl bg-slate-200/70"/><div className="h-48 rounded-2xl bg-slate-200/70"/></div></div>}

function DashboardPage({ data, history, range, onRangeChange, onAddAccount, onOpenAnalytics }: { data: Dashboard | null; history:NetWorthPoint[]; range:string; onRangeChange:(range:string)=>void; onAddAccount: () => void; onOpenAnalytics:()=>void }) {
  if (!data) return null
  const changeSign=data.net_worth_change.amount_minor>=0?'+':''
  const changeText=`${changeSign}${money(data.net_worth_change.amount_minor)}${data.net_worth_change.percent!=null?` · ${changeSign}${data.net_worth_change.percent.toLocaleString('ru-RU')}%`:''} за месяц`
  return <div className="space-y-6">
    <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Metric className="bg-forest text-white" label="Общий капитал" value={money(data.net_worth_minor)} subtitle={changeText} icon={<Landmark size={19}/>} />
      <Metric label="Доход за месяц" value={money(data.month.income_minor)} valueClass="text-emerald-700" icon={<ArrowDownLeft size={19}/>} />
      <Metric label="Расход за месяц" value={money(data.month.expense_minor)} valueClass="text-rose-600" icon={<ArrowUpRight size={19}/>} />
      <Metric label="Денежный поток" value={money(data.month.cash_flow_minor)} valueClass={data.month.cash_flow_minor >= 0 ? 'text-emerald-700' : 'text-rose-600'} icon={<RefreshCw size={18}/>} />
    </section>
    <NetWorthChart data={history} range={range} onRangeChange={onRangeChange}/>
    {data.budget_preview.length>0&&<section className="card p-5"><h2 className="font-bold">Бюджеты</h2><div className="mt-4 grid gap-3 sm:grid-cols-3">{data.budget_preview.map((budget)=><div key={budget.id} className="rounded-xl bg-slate-50 p-3"><div className="flex justify-between text-sm"><b>{budget.category_name}</b><span>{budget.usage_percent.toLocaleString('ru-RU')}%</span></div><div className="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-200"><div className={`h-full ${budget.usage_percent>100?'bg-amber-500':'bg-emerald-600'}`} style={{width:`${Math.min(100,budget.usage_percent)}%`}}/></div></div>)}</div></section>}
    <div className="grid gap-6 xl:grid-cols-2">
      <section className="card p-5"><div className="mb-4 flex items-center justify-between"><div><h2 className="font-bold">Активные цели</h2><p className="mt-1 text-xs text-slate-500">Прогресс обновляется вместе с балансами</p></div><Flag size={18} className="text-slate-400"/></div>{data.active_goals.length?<div className="space-y-3">{data.active_goals.map((goal)=><GoalCard key={goal.id} goal={goal} compact/>)}</div>:<p className="py-10 text-center text-sm text-slate-500">Активных целей пока нет</p>}</section>
      <section className="card p-5"><div className="flex items-start justify-between"><div><h2 className="font-bold">Прогноз капитала</h2><p className="mt-1 text-xs text-slate-500">При текущем среднем темпе</p></div><div className="grid h-10 w-10 place-items-center rounded-xl bg-mint text-forest"><TrendingUp size={19}/></div></div><p className={`mt-6 text-xl font-bold ${data.forecast.average_monthly_growth_minor>=0?'text-emerald-700':'text-rose-600'}`}>{data.forecast.average_monthly_growth_minor>=0?'+':''}{money(data.forecast.average_monthly_growth_minor)}<span className="text-sm font-medium text-slate-400">/мес</span></p><div className="mt-5 grid grid-cols-2 gap-3"><div className="rounded-xl bg-slate-50 p-4"><p className="text-xs text-slate-500">Через 6 месяцев</p><p className="mt-2 font-bold">{money(data.forecast.six_months_minor)}</p></div><div className="rounded-xl bg-slate-50 p-4"><p className="text-xs text-slate-500">Через 12 месяцев</p><p className="mt-2 font-bold">{money(data.forecast.twelve_months_minor)}</p></div></div><button onClick={onOpenAnalytics} className="mt-5 text-sm font-semibold text-emerald-800">Подробная аналитика →</button></section>
    </div>
    <div className="grid gap-6 xl:grid-cols-[0.9fr_1.5fr]">
      <section className="card p-5"><div className="mb-3 flex items-center justify-between"><h2 className="font-bold">Счета</h2><span className="text-xs text-slate-400">{data.accounts.length} активных</span></div>{data.accounts.length ? <div className="space-y-2">{data.accounts.map((account) => <AccountRow key={account.id} account={account}/>)}</div> : <Empty title="Создайте первый счёт" text="Добавьте карту, наличные или накопительный счёт." action="Создать счёт" onAction={onAddAccount}/>}</section>
      <section className="card p-5"><div className="mb-1 flex items-center justify-between"><h2 className="font-bold">Последние операции</h2><span className="text-xs text-slate-400">последние 8</span></div><TransactionList items={data.recent_transactions} compact /></section>
    </div>
  </div>
}

function Metric({ label, value, subtitle, icon, className = 'bg-white', valueClass = '' }: { label: string; value: string; subtitle?:string; icon: React.ReactNode; className?: string; valueClass?: string }) { return <div className={`card p-5 ${className}`}><div className="flex items-center justify-between"><p className={`text-xs font-medium ${className.includes('forest') ? 'text-white/65' : 'text-slate-500'}`}>{label}</p><span className={className.includes('forest') ? 'text-white/70' : 'text-slate-400'}>{icon}</span></div><p className={`mt-5 truncate text-2xl font-bold tracking-tight tabular-nums ${valueClass}`}>{value}</p>{subtitle&&<p className={`mt-2 text-xs ${className.includes('forest')?'text-white/70':'text-slate-500'}`}>{subtitle}</p>}</div> }

function AccountRow({ account }: { account: Account }) { return <div className="flex items-center gap-3 rounded-xl bg-slate-50 p-3.5"><div className="grid h-10 w-10 place-items-center rounded-xl bg-white text-emerald-800 shadow-sm"><CreditCard size={18}/></div><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold">{account.name}</p><p className="text-xs text-slate-500">{accountType[account.type]}</p></div><p className="text-sm font-bold tabular-nums">{money(account.current_balance_minor)}</p></div> }

function AccountsPage({ accounts, onAdd }: { accounts: Account[]; onAdd: () => void }) { return accounts.length ? <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{accounts.map((account) => <article key={account.id} className={`card overflow-hidden p-5 ${!account.is_active ? 'opacity-55' : ''}`}><div className="flex items-start justify-between"><div className="grid h-11 w-11 place-items-center rounded-xl bg-mint text-forest"><CreditCard size={20}/></div><span className={`rounded-full px-2.5 py-1 text-[11px] font-semibold ${account.is_active ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500'}`}>{account.is_active ? 'Активен' : 'Неактивен'}</span></div><h2 className="mt-5 font-bold">{account.name}</h2><p className="mt-1 text-xs text-slate-500">{accountType[account.type]} · {account.currency}</p><p className="mt-5 text-2xl font-bold tracking-tight tabular-nums">{money(account.current_balance_minor)}</p><p className="mt-1 text-xs text-slate-400">Начальный баланс: {money(account.opening_balance_minor)}</p></article>)}</div> : <div className="card"><Empty title="Счетов пока нет" text="Создайте первый счёт, чтобы начать вести финансы." action="Создать счёт" onAction={onAdd}/></div> }

function TransactionsPage({ items,transfers,onDelete,onEdit,onAddTransfer,onDeleteTransfer }: { items: Transaction[];transfers:Transfer[]; onDelete: (id: string) => void;onEdit:(item:Transaction)=>void;onAddTransfer:()=>void;onDeleteTransfer:(id:string)=>void }) { return <div className="space-y-6"><div className="flex justify-end"><button onClick={onAddTransfer} className="flex items-center gap-2 rounded-xl border border-emerald-700 px-4 py-3 text-sm font-semibold text-emerald-800"><ArrowRightLeft size={17}/>Перевод между счетами</button></div><section className="card px-4 py-2 sm:px-6"><TransactionList items={items} onDelete={onDelete} onEdit={onEdit}/></section><section className="card p-5"><h2 className="font-bold">Переводы</h2><div className="mt-3 divide-y">{transfers.map((item)=><div key={item.id} className="flex items-center gap-3 py-3 text-sm"><ArrowRightLeft size={17} className="text-emerald-700"/><div className="min-w-0 flex-1"><p className="font-semibold">{item.from_account_name} → {item.to_account_name}</p><p className="truncate text-xs text-slate-500">{new Date(item.occurred_at).toLocaleString('ru-RU')} · {item.description||'Без комментария'}</p></div><b>{money(item.amount_minor)}</b><button onClick={()=>onDeleteTransfer(item.id)} className="text-xs text-rose-600">Удалить</button></div>)}{!transfers.length&&<p className="py-8 text-center text-sm text-slate-500">Переводов пока нет</p>}</div></section></div> }

function Empty({ title, text, action, onAction }: { title: string; text: string; action: string; onAction: () => void }) { return <div className="py-10 text-center"><div className="mx-auto grid h-12 w-12 place-items-center rounded-2xl bg-mint text-forest"><WalletCards size={22}/></div><h3 className="mt-4 font-semibold">{title}</h3><p className="mx-auto mt-1 max-w-xs text-sm text-slate-500">{text}</p><button onClick={onAction} className="mt-4 text-sm font-semibold text-emerald-800 hover:text-emerald-950">{action} →</button></div> }

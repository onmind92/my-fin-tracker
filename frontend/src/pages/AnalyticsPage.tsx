import { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertCircle, TrendingUp } from 'lucide-react'
import { Bar, BarChart, CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { api } from '../api/client'
import type { CashFlowPoint, CategoryExpense, Forecast, NetWorthPoint } from '../api/types'
import { money } from '../lib/format'

export function AnalyticsPage(){
  const [window,setWindow]=useState(6);const [history,setHistory]=useState<NetWorthPoint[]>([]);const [forecast,setForecast]=useState<Forecast|null>(null);const [cashFlow,setCashFlow]=useState<CashFlowPoint[]>([]);const [categories,setCategories]=useState<CategoryExpense[]>([]);const [loading,setLoading]=useState(true);const [error,setError]=useState('')
  const load=useCallback(async()=>{setError('');try{const [h,f,c,e]=await Promise.all([api.netWorth('1y'),api.forecast(window),api.cashFlow('1y'),api.expensesByCategory('6m')]);setHistory(h);setForecast(f);setCashFlow(c);setCategories(e)}catch(err){setError(err instanceof Error?err.message:'Не удалось загрузить аналитику')}finally{setLoading(false)}},[window])
  // API synchronization when forecast window changes.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(()=>{void load()},[load])
  const combined=useMemo(()=>{const rows:Array<{date:string;history?:number;forecast?:number}>=history.map((p)=>({date:p.date,history:p.net_worth_minor}));if(forecast){if(history.length)rows[rows.length-1].forecast=history[history.length-1].net_worth_minor;for(const p of forecast.forecast)rows.push({date:p.date,forecast:p.net_worth_minor})}return rows},[history,forecast])
  if(loading)return <div className="grid gap-4 sm:grid-cols-2"><div className="h-80 animate-pulse rounded-2xl bg-slate-200"/><div className="h-80 animate-pulse rounded-2xl bg-slate-200"/></div>
  if(error)return <div className="card flex items-center gap-3 p-5 text-rose-700"><AlertCircle/><span>{error}</span></div>
  return <div className="space-y-6">
    <section className="card p-5"><div className="mb-5 flex flex-wrap items-start justify-between gap-4"><div><h2 className="font-bold">Капитал: история и прогноз</h2><p className="mt-1 text-xs text-slate-500">Пунктир — проекция при текущем среднем темпе, не гарантия</p></div><div><span className="mr-2 text-xs text-slate-500">Среднее за:</span><span className="inline-flex rounded-lg bg-slate-100 p-1">{[3,6,12].map((value)=><button key={value} onClick={()=>setWindow(value)} className={`rounded-md px-3 py-1.5 text-xs font-semibold ${window===value?'bg-white text-forest shadow-sm':'text-slate-500'}`}>{value} мес</button>)}</span></div></div>
      {forecast&&<div className="mb-4 flex flex-wrap gap-4 rounded-xl bg-mint px-4 py-3 text-sm"><span className="flex items-center gap-2 font-semibold text-emerald-900"><TrendingUp size={17}/>{money(forecast.average_monthly_growth_minor)}/мес</span><span className="text-emerald-800/70">Использовано полных месяцев: {forecast.actual_window_months} из {forecast.requested_window_months}</span></div>}
      <div className="h-80"><ResponsiveContainer width="100%" height="100%"><ComposedChart data={combined}><CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0"/><XAxis dataKey="date" tick={{fontSize:11}} minTickGap={35}/><YAxis tick={{fontSize:11}} width={65} tickFormatter={(v)=>`${Math.round(v/100000)}k`}/><Tooltip formatter={(value)=>money(Number(value))}/><Legend/><Line name="История" type="monotone" dataKey="history" stroke="#047857" strokeWidth={2.5} dot={false}/><Line name="Прогноз" type="monotone" dataKey="forecast" stroke="#64748b" strokeWidth={2} strokeDasharray="6 5" dot={false}/></ComposedChart></ResponsiveContainer></div>
    </section>
    <div className="grid gap-6 xl:grid-cols-2">
      <section className="card p-5"><h2 className="font-bold">Доходы и расходы</h2><p className="mt-1 text-xs text-slate-500">По завершённым и текущему месяцам</p>{cashFlow.length?<div className="mt-5 h-72"><ResponsiveContainer width="100%" height="100%"><BarChart data={cashFlow}><CartesianGrid strokeDasharray="3 3" vertical={false}/><XAxis dataKey="period" tick={{fontSize:11}}/><YAxis tick={{fontSize:11}} width={58} tickFormatter={(v)=>`${Math.round(v/100000)}k`}/><Tooltip formatter={(value)=>money(Number(value))}/><Legend/><Bar name="Доход" dataKey="income_minor" fill="#059669" radius={[4,4,0,0]}/><Bar name="Расход" dataKey="expense_minor" fill="#fb7185" radius={[4,4,0,0]}/></BarChart></ResponsiveContainer></div>:<EmptyChart text="Операций за период нет"/>}</section>
      <section className="card p-5"><h2 className="font-bold">Расходы по категориям</h2><p className="mt-1 text-xs text-slate-500">Последние 6 месяцев</p>{categories.length?<div className="mt-5 h-72"><ResponsiveContainer width="100%" height="100%"><BarChart data={categories} layout="vertical" margin={{left:10}}><CartesianGrid strokeDasharray="3 3" horizontal={false}/><XAxis type="number" tick={{fontSize:11}} tickFormatter={(v)=>`${Math.round(v/100000)}k`}/><YAxis type="category" dataKey="category_name" width={92} tick={{fontSize:11}}/><Tooltip formatter={(value)=>money(Number(value))}/><Bar dataKey="amount_minor" name="Расход" fill="#0f766e" radius={[0,5,5,0]}/></BarChart></ResponsiveContainer></div>:<EmptyChart text="Расходов за период нет"/>}</section>
    </div>
  </div>
}

function EmptyChart({text}:{text:string}){return <div className="grid h-72 place-items-center text-sm text-slate-500">{text}</div>}

import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { NetWorthPoint } from '../api/types'
import { money } from '../lib/format'

const ranges=[['1m','1M'],['3m','3M'],['6m','6M'],['1y','1Y'],['all','ALL']] as const

export function NetWorthChart({data,range,onRangeChange}: {data:NetWorthPoint[];range:string;onRangeChange:(range:string)=>void}) {
  return <section className="card p-5"><div className="mb-5 flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-bold">История капитала</h2><p className="mt-1 text-xs text-slate-500">Баланс на конец периода</p></div><div className="flex rounded-lg bg-slate-100 p-1">{ranges.map(([value,label])=><button key={value} onClick={()=>onRangeChange(value)} className={`rounded-md px-2.5 py-1.5 text-[11px] font-semibold ${range===value?'bg-white text-forest shadow-sm':'text-slate-500'}`}>{label}</button>)}</div></div>
    {data.length?<div className="h-64 w-full"><ResponsiveContainer width="100%" height="100%"><AreaChart data={data} margin={{top:8,right:8,left:0,bottom:0}}><defs><linearGradient id="netWorthFill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#059669" stopOpacity={0.24}/><stop offset="100%" stopColor="#059669" stopOpacity={0}/></linearGradient></defs><CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0"/><XAxis dataKey="date" tick={{fontSize:11,fill:'#94a3b8'}} tickLine={false} axisLine={false} minTickGap={28} tickFormatter={(v)=>new Date(v).toLocaleDateString('ru-RU',{day:'numeric',month:'short'})}/><YAxis tick={{fontSize:11,fill:'#94a3b8'}} tickLine={false} axisLine={false} width={62} tickFormatter={(v)=>`${Math.round(v/100000)}k`}/><Tooltip formatter={(value)=>money(Number(value))} labelFormatter={(value)=>new Date(String(value)).toLocaleDateString('ru-RU')}/><Area type="monotone" dataKey="net_worth_minor" stroke="#047857" strokeWidth={2.5} fill="url(#netWorthFill)"/></AreaChart></ResponsiveContainer></div>:<div className="grid h-64 place-items-center text-sm text-slate-500">История появится после создания счёта</div>}
  </section>
}

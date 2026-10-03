import { Archive, CalendarDays, Edit3, Flag } from 'lucide-react'
import type { Goal } from '../api/types'
import { money } from '../lib/format'

export function GoalCard({ goal, compact=false, onEdit, onArchive }: { goal:Goal; compact?:boolean; onEdit?:(goal:Goal)=>void; onArchive?:(goal:Goal)=>void }) {
  const deadline=goal.deadline?new Intl.DateTimeFormat('ru-RU',{day:'numeric',month:'short',year:'numeric'}).format(new Date(goal.deadline)):null
  return <article className={`${compact?'rounded-xl bg-slate-50 p-4':'card p-5'}`}>
    <div className="flex items-start gap-3"><div className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-mint text-forest"><Flag size={18}/></div><div className="min-w-0 flex-1"><div className="flex items-start justify-between gap-2"><div><h3 className="truncate font-semibold">{goal.name}</h3><p className="mt-0.5 text-xs text-slate-500">{goal.type==='global'?'Общий капитал':goal.linked_account_name?`Счёт: ${goal.linked_account_name}`:'Ручной прогресс'}</p></div>{!compact&&<div className="flex"><button onClick={()=>onEdit?.(goal)} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-ink" aria-label="Редактировать"><Edit3 size={16}/></button><button onClick={()=>onArchive?.(goal)} className="rounded-lg p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600" aria-label="Архивировать"><Archive size={16}/></button></div>}</div>
      <div className="mt-4 flex items-end justify-between gap-2"><p className="text-sm"><strong>{money(goal.current_amount_minor)}</strong><span className="text-slate-400"> / {money(goal.target_amount_minor)}</span></p><span className="text-xs font-semibold text-emerald-700">{goal.progress_percent.toLocaleString('ru-RU')}%</span></div>
      <div className="mt-2 h-2 overflow-hidden rounded-full bg-slate-200"><div className="h-full rounded-full bg-emerald-600 transition-all" style={{width:`${goal.progress_percent}%`}}/></div>
      {!compact&&<div className="mt-4 grid grid-cols-2 gap-3 text-xs"><div><span className="text-slate-400">Осталось</span><p className="mt-1 font-semibold">{money(goal.remaining_amount_minor)}</p></div><div><span className="text-slate-400">Нужный темп</span><p className="mt-1 font-semibold">{goal.required_monthly_amount_minor!=null?`${money(goal.required_monthly_amount_minor)}/мес`:'—'}</p></div></div>}
      {!compact&&deadline&&<p className={`mt-4 flex items-center gap-1.5 text-xs ${goal.days_remaining!=null&&goal.days_remaining<0?'text-rose-600':'text-slate-500'}`}><CalendarDays size={14}/>{goal.days_remaining!=null&&goal.days_remaining<0?'Срок прошёл: ':'Срок: '}{deadline}</p>}
      {!compact&&<p className="mt-2 text-xs text-slate-500">{goal.reached?'Цель достигнута':goal.estimated_completion_date?`Оценка достижения: ${new Intl.DateTimeFormat('ru-RU',{month:'long',year:'numeric'}).format(new Date(goal.estimated_completion_date))}`:'При текущем темпе дата достижения не прогнозируется'}</p>}
    </div></div>
  </article>
}


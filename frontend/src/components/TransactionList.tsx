import { ArrowDownLeft, ArrowUpRight, Trash2 } from 'lucide-react'
import type { Transaction } from '../api/types'
import { date, money } from '../lib/format'

export function TransactionList({ items, onDelete, onEdit, compact = false }: { items: Transaction[]; onDelete?: (id: string) => void; onEdit?:(item:Transaction)=>void; compact?: boolean }) {
  if (!items.length) return <div className="py-12 text-center text-sm text-slate-500">Операций пока нет</div>
  return <div className="divide-y divide-slate-100">
    {items.map((item) => {
      const income = item.type === 'income'
      return <div key={item.id} onClick={()=>onEdit?.(item)} className={`group flex items-center gap-3 py-3.5 ${onEdit?'cursor-pointer rounded-xl px-2 hover:bg-slate-50':''}`}>
        <div className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl ${income ? 'bg-emerald-50 text-emerald-700' : 'bg-rose-50 text-rose-600'}`}>
          {income ? <ArrowDownLeft size={18} /> : <ArrowUpRight size={18} />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span>{item.category_icon}</span><p className="truncate text-sm font-semibold">{item.description || item.category_name || 'Без описания'}</p>
          </div>
          <p className="mt-0.5 truncate text-xs text-slate-500">{item.account_name} · {date(item.occurred_at)}</p>
        </div>
        <div className="flex items-center gap-2">
          <span className={`whitespace-nowrap text-sm font-semibold tabular-nums ${income ? 'text-emerald-700' : 'text-ink'}`}>{income ? '+' : '−'}{money(item.amount_minor)}</span>
          {!compact && onDelete && <button onClick={(event) => {event.stopPropagation();onDelete(item.id)}} className="rounded-lg p-2 text-slate-300 opacity-0 transition hover:bg-rose-50 hover:text-rose-600 group-hover:opacity-100 focus:opacity-100" aria-label="Удалить операцию"><Trash2 size={16} /></button>}
        </div>
      </div>
    })}
  </div>
}

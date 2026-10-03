import { useState } from 'react'
import { X } from 'lucide-react'
import type { Account, Goal, GoalInput, GoalType } from '../api/types'

export function GoalModal({ accounts, goal, onClose, onSubmit }: { accounts: Account[]; goal?: Goal; onClose: () => void; onSubmit: (input: GoalInput) => Promise<void> }) {
  const initialSource = goal?.linked_account_id ? 'account' : 'manual'
  const [name,setName]=useState(goal?.name || '')
  const [type,setType]=useState<GoalType>(goal?.type || 'global')
  const [target,setTarget]=useState(goal ? String(goal.target_amount_minor/100) : '')
  const [deadline,setDeadline]=useState(goal?.deadline?.slice(0,10) || '')
  const [source,setSource]=useState<'manual'|'account'>(initialSource)
  const [accountId,setAccountId]=useState(goal?.linked_account_id || accounts.find((item)=>item.is_active)?.id || '')
  const [manual,setManual]=useState(goal?.manual_current_amount_minor != null ? String(goal.manual_current_amount_minor/100) : '0')
  const [error,setError]=useState('');const [saving,setSaving]=useState(false)
  async function submit(event:React.FormEvent){event.preventDefault();const targetValue=Number(target.replace(',','.'));const manualValue=Number(manual.replace(',','.'));if(!name.trim()){setError('Укажите название цели');return}if(!Number.isFinite(targetValue)||targetValue<=0){setError('Целевая сумма должна быть больше нуля');return}if(type==='current'&&source==='account'&&!accountId){setError('Выберите активный счёт');return}if(type==='current'&&source==='manual'&&(!Number.isFinite(manualValue)||manualValue<0)){setError('Текущая сумма не может быть отрицательной');return}
    const input:GoalInput={name:name.trim(),type,target_amount_minor:Math.round(targetValue*100),deadline,status:goal?.status||'active'}
    if(type==='current'&&source==='account')input.linked_account_id=accountId
    if(type==='current'&&source==='manual')input.manual_current_amount_minor=Math.round(manualValue*100)
    setSaving(true);setError('');try{await onSubmit(input)}catch(err){setError(err instanceof Error?err.message:'Не удалось сохранить цель');setSaving(false)}
  }
  return <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/35 backdrop-blur-[2px] sm:items-center sm:p-6" onMouseDown={(e)=>e.target===e.currentTarget&&onClose()}><form onSubmit={submit} className="max-h-[92vh] w-full max-w-lg overflow-y-auto rounded-t-3xl bg-white p-6 shadow-2xl sm:rounded-3xl">
    <div className="mb-6 flex items-center justify-between"><div><h2 className="text-xl font-bold">{goal?'Редактировать цель':'Новая цель'}</h2><p className="mt-1 text-sm text-slate-500">Прогресс и темп рассчитаются автоматически</p></div><button type="button" onClick={onClose} className="rounded-xl p-2 text-slate-500 hover:bg-slate-100"><X size={20}/></button></div>
    <div className="space-y-4"><label><span className="label">Название</span><input autoFocus className="field" value={name} onChange={(e)=>setName(e.target.value)} placeholder="Накопить на поездку"/></label>
      <label><span className="label">Тип цели</span><select className="field" value={type} onChange={(e)=>setType(e.target.value as GoalType)}><option value="global">Общий капитал</option><option value="current">Отдельная цель</option></select></label>
      <label><span className="label">Целевая сумма, ₽</span><input className="field" inputMode="decimal" value={target} onChange={(e)=>setTarget(e.target.value)} placeholder="100000"/></label>
      <label><span className="label">Срок — необязательно</span><input type="date" className="field" value={deadline} onChange={(e)=>setDeadline(e.target.value)}/></label>
      {type==='current'&&<><div><span className="label">Источник прогресса</span><div className="grid grid-cols-2 rounded-xl bg-slate-100 p-1"><button type="button" onClick={()=>setSource('manual')} className={`rounded-lg py-2.5 text-sm font-semibold ${source==='manual'?'bg-white shadow-sm':'text-slate-500'}`}>Вручную</button><button type="button" onClick={()=>setSource('account')} className={`rounded-lg py-2.5 text-sm font-semibold ${source==='account'?'bg-white shadow-sm':'text-slate-500'}`}>По счёту</button></div></div>
        {source==='account'?<label><span className="label">Связанный счёт</span><select className="field" value={accountId} onChange={(e)=>setAccountId(e.target.value)}><option value="">Выберите счёт</option>{accounts.filter((a)=>a.is_active).map((a)=><option key={a.id} value={a.id}>{a.name}</option>)}</select></label>:<label><span className="label">Уже накоплено, ₽</span><input className="field" inputMode="decimal" value={manual} onChange={(e)=>setManual(e.target.value)}/></label>}</>}
    </div>{error&&<p className="mt-4 rounded-xl bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>}<button disabled={saving} className="mt-6 w-full rounded-xl bg-forest px-4 py-3.5 text-sm font-semibold text-white disabled:opacity-50">{saving?'Сохраняем…':'Сохранить цель'}</button>
  </form></div>
}

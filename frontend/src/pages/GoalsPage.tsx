import { Flag, Plus } from 'lucide-react'
import type { Goal } from '../api/types'
import { GoalCard } from '../components/GoalCard'

export function GoalsPage({goals,onCreate,onEdit,onArchive}:{goals:Goal[];onCreate:()=>void;onEdit:(goal:Goal)=>void;onArchive:(goal:Goal)=>void}){
  const active=goals.filter((goal)=>goal.status==='active')
  return active.length?<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{active.map((goal)=><GoalCard key={goal.id} goal={goal} onEdit={onEdit} onArchive={onArchive}/>)}</div>:<div className="card py-16 text-center"><div className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-mint text-forest"><Flag size={24}/></div><h2 className="mt-4 font-bold">Добавьте первую цель</h2><p className="mx-auto mt-2 max-w-sm text-sm text-slate-500">Следите за общим капиталом или копите конкретную сумму вручную либо на отдельном счёте.</p><button onClick={onCreate} className="mt-5 inline-flex items-center gap-2 rounded-xl bg-forest px-4 py-3 text-sm font-semibold text-white"><Plus size={17}/>Создать цель</button></div>
}


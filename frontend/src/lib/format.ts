export const money = (minor: number) => new Intl.NumberFormat('ru-RU', {
  style: 'currency', currency: 'RUB', maximumFractionDigits: 2,
}).format(minor / 100)

export const date = (value: string) => new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
}).format(new Date(value))

export const accountType: Record<string, string> = {
  card: 'Карта', cash: 'Наличные', savings: 'Накопления', investment: 'Инвестиции', other: 'Другой',
}

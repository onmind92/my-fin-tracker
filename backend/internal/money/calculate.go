package money

import "fmt"

type Entry struct {
	Type        string
	AmountMinor int64
}

func Apply(opening int64, entries []Entry) (int64, error) {
	balance := opening
	for _, entry := range entries {
		if entry.AmountMinor <= 0 {
			return 0, fmt.Errorf("amount_minor must be greater than zero")
		}
		switch entry.Type {
		case "income":
			balance += entry.AmountMinor
		case "expense":
			balance -= entry.AmountMinor
		default:
			return 0, fmt.Errorf("unsupported transaction type %q", entry.Type)
		}
	}
	return balance, nil
}

func NetWorth(balances []int64) int64 {
	var total int64
	for _, balance := range balances {
		total += balance
	}
	return total
}

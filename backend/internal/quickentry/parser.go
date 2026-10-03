package quickentry

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Category struct{ ID, Name, Type string }
type Account struct {
	ID, Name, Type string
	Active         bool
}
type Catalog struct {
	Categories []Category
	Accounts   []Account
}

type Transaction struct {
	Type         string     `json:"type"`
	AmountMinor  *int64     `json:"amount_minor,omitempty"`
	CategoryID   *string    `json:"category_id,omitempty"`
	CategoryName *string    `json:"category_name,omitempty"`
	AccountID    *string    `json:"account_id,omitempty"`
	AccountName  *string    `json:"account_name,omitempty"`
	Description  string     `json:"description,omitempty"`
	OccurredAt   *time.Time `json:"occurred_at,omitempty"`
}
type Result struct {
	Parsed      bool        `json:"parsed"`
	Confidence  string      `json:"confidence"`
	Transaction Transaction `json:"transaction"`
	Unresolved  []string    `json:"unresolved"`
}

var amountPattern = regexp.MustCompile(`(?:^|\s)(\d+(?:[.,]\d{1,2})?)(?:\s|$)`)
var categoryRules = []struct {
	Name, Type string
	Words      []string
}{
	{"Salary", "income", []string{"зарплата", "зарплату", "оклад"}},
	{"Freelance", "income", []string{"фриланс", "подработка", "гонорар"}},
	{"Gift", "income", []string{"подарок", "подарили"}},
	{"Food", "expense", []string{"кофе", "еда", "продукты", "продукт", "обед", "ужин", "завтрак", "кафе", "ресторан"}},
	{"Transport", "expense", []string{"такси", "метро", "автобус", "транспорт", "бензин"}},
	{"Shopping", "expense", []string{"покупка", "покупки", "магазин", "одежда"}},
	{"Entertainment", "expense", []string{"кино", "развлечения", "концерт", "игра"}},
	{"Health", "expense", []string{"аптека", "врач", "здоровье", "лекарства"}},
}

func Parse(text string, catalog Catalog, now time.Time) Result {
	clean := normalize(text)
	result := Result{Confidence: "partial", Unresolved: make([]string, 0), Transaction: Transaction{Type: "expense"}}
	if clean == "" {
		result.Confidence = "none"
		result.Unresolved = []string{"text", "amount", "category"}
		return result
	}
	result.Parsed = true
	match := amountPattern.FindStringSubmatch(clean)
	if len(match) > 1 {
		value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", "."), 64)
		if err == nil && value > 0 {
			minor := int64(value*100 + 0.5)
			result.Transaction.AmountMinor = &minor
		} else {
			result.Unresolved = append(result.Unresolved, "amount")
		}
	} else {
		result.Unresolved = append(result.Unresolved, "amount")
	}
	for _, rule := range categoryRules {
		if containsWord(clean, rule.Words) {
			result.Transaction.Type = rule.Type
			for _, category := range catalog.Categories {
				if strings.EqualFold(category.Name, rule.Name) && category.Type == rule.Type {
					id, name := category.ID, category.Name
					result.Transaction.CategoryID = &id
					result.Transaction.CategoryName = &name
					break
				}
			}
			break
		}
	}
	if result.Transaction.CategoryID == nil {
		result.Unresolved = append(result.Unresolved, "category")
	}
	if strings.Contains(clean, "вчера") {
		value := now.AddDate(0, 0, -1)
		result.Transaction.OccurredAt = &value
	}
	for _, account := range catalog.Accounts {
		if !account.Active {
			continue
		}
		name := normalize(account.Name)
		matched := name != "" && strings.Contains(clean, name)
		if !matched && strings.Contains(clean, "карта") && account.Type == "card" {
			matched = true
		}
		if !matched && strings.Contains(clean, "наличные") && account.Type == "cash" {
			matched = true
		}
		if matched {
			id, label := account.ID, account.Name
			result.Transaction.AccountID = &id
			result.Transaction.AccountName = &label
			break
		}
	}
	description := amountPattern.ReplaceAllString(clean, " ")
	description = strings.ReplaceAll(description, "вчера", "")
	description = strings.TrimSpace(strings.Join(strings.Fields(description), " "))
	result.Transaction.Description = description
	if len(result.Unresolved) == 0 {
		result.Confidence = "high"
	}
	return result
}

func normalize(value string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == 'ё' {
			return 'е'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == ',' || r == '.' {
			return unicode.ToLower(r)
		}
		return ' '
	}, strings.TrimSpace(value)))
}
func containsWord(text string, words []string) bool {
	fields := strings.Fields(text)
	for _, field := range fields {
		for _, word := range words {
			if field == word {
				return true
			}
		}
	}
	return false
}

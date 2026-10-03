package importdata

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strconv"
	"strings"
	"time"
)

type Mapping struct {
	DateColumn        string `json:"date_column"`
	AmountColumn      string `json:"amount_column"`
	TypeColumn        string `json:"type_column"`
	DescriptionColumn string `json:"description_column"`
	CategoryColumn    string `json:"category_column"`
	AccountID         string `json:"account_id"`
}
type Parsed struct {
	Index       int       `json:"index"`
	Date        time.Time `json:"date"`
	Type        string    `json:"type"`
	AmountMinor int64     `json:"amount_minor"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
}

func Read(data []byte) ([]string, [][]string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	comma := ','
	first := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		first = data[:i]
	}
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		comma = ';'
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(rows) < 1 {
		return nil, nil, errors.New("CSV is empty")
	}
	headers := rows[0]
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	return headers, rows[1:], nil
}
func Parse(headers []string, rows [][]string, m Mapping, location *time.Location) ([]Parsed, []string) {
	index := map[string]int{}
	for i, h := range headers {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{m.DateColumn, m.AmountColumn}
	for _, name := range required {
		if _, ok := index[strings.ToLower(name)]; !ok {
			return nil, []string{"required mapped column not found: " + name}
		}
	}
	result := []Parsed{}
	issues := []string{}
	value := func(row []string, name string) string {
		if name == "" {
			return ""
		}
		i, ok := index[strings.ToLower(name)]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	for rowIndex, row := range rows {
		date, err := parseDate(value(row, m.DateColumn), location)
		if err != nil {
			issues = append(issues, "row "+strconv.Itoa(rowIndex+2)+": invalid date")
			continue
		}
		amount, err := parseAmount(value(row, m.AmountColumn))
		if err != nil || amount == 0 {
			issues = append(issues, "row "+strconv.Itoa(rowIndex+2)+": invalid amount")
			continue
		}
		kind := strings.ToLower(value(row, m.TypeColumn))
		if kind == "" {
			if amount < 0 {
				kind = "expense"
			} else {
				kind = "income"
			}
		}
		if kind != "income" && kind != "expense" {
			issues = append(issues, "row "+strconv.Itoa(rowIndex+2)+": invalid type")
			continue
		}
		if amount < 0 {
			amount = -amount
		}
		result = append(result, Parsed{Index: rowIndex, Date: date, Type: kind, AmountMinor: amount, Description: value(row, m.DescriptionColumn), Category: value(row, m.CategoryColumn)})
	}
	return result, issues
}
func parseDate(raw string, location *time.Location) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "02.01.2006", "02/01/2006", time.RFC3339} {
		if value, err := time.ParseInLocation(layout, raw, location); err == nil {
			return value, nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}
func parseAmount(raw string) (int64, error) {
	value := strings.NewReplacer(" ", "", "\u00a0", "", "₽", "", "RUB", "", "rub", "", ",", ".").Replace(strings.TrimSpace(raw))
	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "+")
	value = strings.TrimPrefix(value, "-")
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid amount")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	fraction := int64(0)
	if len(parts) == 2 {
		digits := parts[1]
		if len(digits) > 2 {
			return 0, errors.New("too many decimals")
		}
		if len(digits) == 1 {
			digits += "0"
		}
		if digits != "" {
			fraction, err = strconv.ParseInt(digits, 10, 64)
			if err != nil {
				return 0, err
			}
		}
	}
	minor := whole*100 + fraction
	if negative {
		minor = -minor
	}
	return minor, nil
}
func PreviewRows(rows [][]string, limit int) [][]string {
	if limit > len(rows) {
		limit = len(rows)
	}
	if limit < 0 {
		limit = 0
	}
	return rows[:limit]
}

package quickentry

import (
	"testing"
	"time"
)

func TestParserExamples(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	catalog := testCatalog()
	tests := []struct {
		text, kind, category string
		amount               int64
		yesterday            bool
	}{
		{"кофе 350", "expense", "Food", 35000, false},
		{"такси 740 вчера", "expense", "Transport", 74000, true},
		{"зарплата 50000", "income", "Salary", 5000000, false},
	}
	for _, tt := range tests {
		got := Parse(tt.text, catalog, now)
		if !got.Parsed || got.Transaction.AmountMinor == nil || *got.Transaction.AmountMinor != tt.amount || got.Transaction.Type != tt.kind || got.Transaction.CategoryName == nil || *got.Transaction.CategoryName != tt.category {
			t.Fatalf("%q: %+v", tt.text, got)
		}
		if tt.yesterday && (got.Transaction.OccurredAt == nil || got.Transaction.OccurredAt.Day() != 1) {
			t.Fatalf("yesterday not parsed: %+v", got)
		}
	}
}
func TestParserMissingAmount(t *testing.T) {
	got := Parse("кофе", testCatalog(), time.Now())
	if got.Confidence != "partial" || !has(got.Unresolved, "amount") {
		t.Fatalf("%+v", got)
	}
}
func TestParserUnknownCategory(t *testing.T) {
	got := Parse("неизвестное 500", testCatalog(), time.Now())
	if !has(got.Unresolved, "category") {
		t.Fatalf("%+v", got)
	}
}
func TestParserInvalidInput(t *testing.T) {
	got := Parse("!!!", testCatalog(), time.Now())
	if got.Parsed || got.Confidence != "none" {
		t.Fatalf("%+v", got)
	}
}
func testCatalog() Catalog {
	return Catalog{Categories: []Category{{ID: "food", Name: "Food", Type: "expense"}, {ID: "transport", Name: "Transport", Type: "expense"}, {ID: "salary", Name: "Salary", Type: "income"}}}
}
func has(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

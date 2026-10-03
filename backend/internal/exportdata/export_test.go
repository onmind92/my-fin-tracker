package exportdata

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"finance-tracker/backend/internal/domain"
)

func TestCSVFormattingEscapesUnicodeQuotesAndNewlines(t *testing.T) {
	category := "Food, café"
	description := "Кофе \"большой\"\nутром"
	items := []domain.Transaction{{Type: "expense", AmountMinor: 35050, AccountName: "Main, card", CategoryName: &category, Description: &description, OccurredAt: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)}}
	var buffer bytes.Buffer
	if err := WriteTransactionsCSV(&buffer, items, time.UTC); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(buffer.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][2] != "350.50" || records[1][5] != category || records[1][6] != description {
		t.Fatalf("bad csv records: %#v", records)
	}
}
func TestJSONArchiveExcludesSensitiveFields(t *testing.T) {
	archive := Archive{ExportVersion: 1, Data: Data{User: domain.User{Email: "user@example.com", Timezone: "UTC"}}}
	raw, err := json.Marshal(archive)
	if err != nil {
		t.Fatal(err)
	}
	value := string(raw)
	for _, forbidden := range []string{"password_hash", "session", "token_hash"} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("archive leaked %s", forbidden)
		}
	}
}

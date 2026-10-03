package importdata

import (
	"testing"
	"time"
)

func TestCSVSemicolonBOMAndSignedAmounts(t *testing.T) {
	headers, rows, err := Read(append([]byte{0xEF, 0xBB, 0xBF}, []byte("date;amount;description\n01.10.2026;-1234,56;Кафе\n")...))
	if err != nil {
		t.Fatal(err)
	}
	parsed, issues := Parse(headers, rows, Mapping{DateColumn: "date", AmountColumn: "amount", DescriptionColumn: "description", AccountID: "x"}, time.UTC)
	if len(issues) != 0 || len(parsed) != 1 || parsed[0].Type != "expense" || parsed[0].AmountMinor != 123456 {
		t.Fatalf("%+v %+v", parsed, issues)
	}
}

func TestCSVInvalidRowsAreReported(t *testing.T) {
	headers, rows, err := Read([]byte("date,amount,type\nwrong,0,expense\n2026-10-01,10,unknown\n"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, issues := Parse(headers, rows, Mapping{DateColumn: "date", AmountColumn: "amount", TypeColumn: "type"}, time.UTC)
	if len(parsed) != 0 || len(issues) != 2 {
		t.Fatalf("parsed=%+v issues=%+v", parsed, issues)
	}
}

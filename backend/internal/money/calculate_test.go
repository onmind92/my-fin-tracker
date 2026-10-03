package money

import "testing"

func TestApply(t *testing.T) {
	got, err := Apply(5_000_000, []Entry{
		{Type: "expense", AmountMinor: 100_000},
		{Type: "expense", AmountMinor: 50_000},
		{Type: "income", AmountMinor: 2_000_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(6_850_000); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestApplyIncomeAndExpense(t *testing.T) {
	got, err := Apply(1_000, []Entry{{Type: "income", AmountMinor: 500}, {Type: "expense", AmountMinor: 200}})
	if err != nil || got != 1_300 {
		t.Fatalf("got %d, err %v", got, err)
	}
}

func TestNetWorth(t *testing.T) {
	if got := NetWorth([]int64{100_000, 250_000, -50_000}); got != 300_000 {
		t.Fatalf("got %d, want 300000", got)
	}
}

func TestApplyRejectsNonPositiveAmount(t *testing.T) {
	for _, amount := range []int64{0, -1} {
		if _, err := Apply(0, []Entry{{Type: "income", AmountMinor: amount}}); err == nil {
			t.Fatalf("expected error for amount %d", amount)
		}
	}
}

package store

import "testing"

func TestValidateTemplateValidAndNullableAmount(t *testing.T) {
	if err := validateTemplate(TemplateInput{Name: "Coffee", TransactionType: "expense", AmountMinor: nil}); err != nil {
		t.Fatal(err)
	}
	amount := int64(35000)
	if err := validateTemplate(TemplateInput{Name: "Coffee", TransactionType: "expense", AmountMinor: &amount}); err != nil {
		t.Fatal(err)
	}
}
func TestValidateTemplateRejectsInvalidAmount(t *testing.T) {
	amount := int64(0)
	if err := validateTemplate(TemplateInput{Name: "Coffee", TransactionType: "expense", AmountMinor: &amount}); err == nil {
		t.Fatal("expected validation error")
	}
}
func TestReferenceOwnershipDecision(t *testing.T) {
	if err := validateReferenceState(true, false, true, true, true, "expense", "expense"); err == nil {
		t.Fatal("foreign account must be rejected")
	}
}

package finance

import "testing"

func TestClassify_OverrideWinsOverEverything(t *testing.T) {
	// A negative on-budget purchase normally classifies as spent. With
	// override "saved" it must go to saved (e.g. user marked their
	// rent as a forced-savings discipline).
	got := Classify(ClassifyInput{
		Amount:           -100000,
		AccountOnBudget:  true,
		FlowOverride:     "saved",
		Category:         "Rent",
		AccountName:      "Chase Checking",
		AccountType:      "checking",
	})
	if got != FlowSaved {
		t.Errorf("override saved should win; got %s", got)
	}
}

func TestClassify_OverrideExcludeKillsRogueRow(t *testing.T) {
	got := Classify(ClassifyInput{
		Amount:          55079,
		AccountOnBudget: true,
		IncludeInIncome: true,
		Payee:           "Payment",
		Category:        "",
		FlowOverride:    "exclude",
	})
	if got != FlowExclude {
		t.Errorf("override exclude should drop the row; got %s", got)
	}
}

func TestClassify_StartingBalanceVariantsExcluded(t *testing.T) {
	for _, payee := range []string{
		"Starting Balance",
		"Initial Balance Import",
		"Opening Balance",
	} {
		got := Classify(ClassifyInput{
			Amount: 5000000, Payee: payee,
			IncludeInIncome: true, IsSavingsDest: true,
		})
		if got != FlowExclude {
			t.Errorf("payee %q should exclude; got %s", payee, got)
		}
	}
}

func TestClassify_CCPaymentExcluded(t *testing.T) {
	// The destination leg of a CC payment lands on the credit card
	// account as positive; the source leg lands on checking as negative.
	// Both must be excluded regardless of account flags.
	for _, cat := range []string{"Credit Card Payment", "Payment", "credit card payment"} {
		got := Classify(ClassifyInput{
			Amount: 200000, Category: cat,
			IncludeInIncome: true, AccountOnBudget: true,
		})
		if got != FlowExclude {
			t.Errorf("category %q positive should exclude; got %s", cat, got)
		}
		got = Classify(ClassifyInput{
			Amount: -200000, Category: cat,
			AccountOnBudget: true,
		})
		if got != FlowExclude {
			t.Errorf("category %q negative should exclude; got %s", cat, got)
		}
	}
}

func TestClassify_SplitParentExcluded(t *testing.T) {
	got := Classify(ClassifyInput{
		Amount: -30000, IsParent: true,
		AccountOnBudget: true,
	})
	if got != FlowExclude {
		t.Errorf("split parent should exclude; got %s", got)
	}
}

func TestClassify_TransferToSavingsIsSaved(t *testing.T) {
	got := Classify(ClassifyInput{
		Amount: 100000, TransferID: "peer-1",
		IsSavingsDest: true, AccountType: "savings",
		Payee: "Transfer from Chase",
	})
	if got != FlowSaved {
		t.Errorf("transfer to savings → saved; got %s", got)
	}
}

func TestClassify_TransferBetweenNonSavingsExcluded(t *testing.T) {
	got := Classify(ClassifyInput{
		Amount: 200000, TransferID: "peer-2",
		IsSavingsDest: false, IncludeInIncome: true,
		AccountOnBudget: true,
	})
	if got != FlowExclude {
		t.Errorf("non-savings transfer leg → exclude; got %s", got)
	}
}

func TestClassify_BalanceImportOnSavingsExcluded(t *testing.T) {
	// Bare deposit row on a savings destination with no transfer link
	// and a payee that matches the account name = balance import.
	got := Classify(ClassifyInput{
		Amount: 5000000, IsSavingsDest: true,
		AccountName: "Robinhood", Payee: "Robinhood",
	})
	if got != FlowExclude {
		t.Errorf("balance-import row should exclude; got %s", got)
	}
	// And the empty-payee variant.
	got = Classify(ClassifyInput{
		Amount: 5000000, IsSavingsDest: true,
		AccountName: "Robinhood", Payee: "",
	})
	if got != FlowExclude {
		t.Errorf("empty-payee balance row should exclude; got %s", got)
	}
}

func TestClassify_EmployerDepositToSavingsIsSaved(t *testing.T) {
	// Paycheck → 401K direct deposit has no transfer_id (employer
	// isn't an Actual account) but has a real payee. Must count as
	// saved, not excluded.
	got := Classify(ClassifyInput{
		Amount: 100000, IsSavingsDest: true,
		AccountName: "Fidelity 401K", Payee: "ACME Corp Payroll",
		AccountType: "investment",
	})
	if got != FlowSaved {
		t.Errorf("paycheck deposit to 401K → saved; got %s", got)
	}
}

func TestClassify_PaycheckToCheckingIsIncome(t *testing.T) {
	got := Classify(ClassifyInput{
		Amount: 520000, Payee: "ACME Corp",
		AccountName: "Chase Checking", AccountType: "checking",
		IncludeInIncome: true,
	})
	if got != FlowIncome {
		t.Errorf("paycheck to checking → income; got %s", got)
	}
}

func TestClassify_AmexCheckingIncomeStillCounts(t *testing.T) {
	// User report: Amex Checking is off-budget in Actual but still
	// receives a paycheck. AccountOnBudget=false must not block income.
	got := Classify(ClassifyInput{
		Amount: 300000, Payee: "ACME Corp",
		AccountName: "Amex Checking", AccountType: "checking",
		AccountOnBudget: false, IncludeInIncome: true,
	})
	if got != FlowIncome {
		t.Errorf("off-budget Amex Checking paycheck → income; got %s", got)
	}
}

func TestClassify_OutflowOffBudgetExcluded(t *testing.T) {
	// Negative on an off-budget investment account (e.g. brokerage
	// withdrawal) isn't real spending — exclude.
	got := Classify(ClassifyInput{
		Amount: -50000, AccountName: "Robinhood",
		AccountType: "investment", AccountOnBudget: false,
	})
	if got != FlowExclude {
		t.Errorf("off-budget outflow should exclude; got %s", got)
	}
}

func TestDefaultIncludeInIncome(t *testing.T) {
	cases := map[string]bool{
		"Chase Checking":          true,
		"Amex Checking":           true,
		"Chase Savings":           true,
		"Robinhood":               true,
		"Fidelity 401K":           true,
		"Amex Platinum":           false, // credit card
		"Chase Sapphire Reserve":  false,
		"Apple Card":              false,
		"Mortgage":                false, // type heuristic
	}
	for name, want := range cases {
		acctType := ""
		switch name {
		case "Mortgage":
			acctType = "mortgage"
		case "Amex Platinum", "Chase Sapphire Reserve", "Apple Card":
			acctType = "credit"
		}
		got := DefaultIncludeInIncome(acctType, name)
		if got != want {
			t.Errorf("DefaultIncludeInIncome(%q, %q) = %v, want %v", acctType, name, got, want)
		}
	}
}

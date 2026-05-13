package finance

import "testing"

func TestDefaultGroupFor(t *testing.T) {
	// Maps real categories from the user's Actual taxonomy to expected groups.
	cases := map[string]string{
		// Living Expenses
		"Mortgage":         GroupLiving,
		"Utilities":        GroupLiving,
		"Phone & Internet": GroupLiving,
		"Home":             GroupLiving,
		"Groceries":        GroupLiving,
		"Medical":          GroupLiving,
		"Insurance":        GroupLiving,
		"Education":        GroupLiving,
		"Pet Care":         GroupLiving,
		// Transportation
		"Gas":                 GroupTrans,
		"Vehicle Maintenance": GroupTrans,
		// Dining & Entertainment
		"Restaurants":     GroupDining,
		"Entertainment":   GroupDining,
		"Subscriptions":   GroupDining,
		"Travel":          GroupDining,
		"Gifts":           GroupDining,
		"Personal Care":   GroupDining,
		"Online Shopping": GroupDining,
		// Savings & Income
		"Salary":             GroupSavings,
		"Rental Income":      GroupSavings,
		"Other Income":       GroupSavings,
		"Reimbursement":      GroupSavings,
		"Starting Balances":  GroupSavings,
		// Misc fallback
		"Cash":                     GroupMisc,
		"Government Fees":          GroupMisc,
		"Personal Transfers (P2P)": GroupSavings, // "transfer" keyword wins
		"Uncategorized":            GroupMisc,
		"":                         GroupMisc,
		"Something random":         GroupMisc,
	}
	for cat, want := range cases {
		t.Run(cat, func(t *testing.T) {
			got := DefaultGroupFor(cat)
			if got != want {
				t.Errorf("DefaultGroupFor(%q) = %q, want %q", cat, got, want)
			}
		})
	}
}

func TestAllGroupsOrderIsStable(t *testing.T) {
	// Order matters — iOS renders sections in this order. Locking the
	// sequence catches accidental reorderings.
	want := []string{
		GroupLiving, GroupTrans, GroupDining, GroupSavings, GroupMisc,
	}
	if len(AllGroups) != len(want) {
		t.Fatalf("AllGroups len = %d, want %d", len(AllGroups), len(want))
	}
	for i := range want {
		if AllGroups[i] != want[i] {
			t.Errorf("AllGroups[%d] = %q, want %q", i, AllGroups[i], want[i])
		}
	}
}

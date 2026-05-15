package finance

import (
	"strings"
)

// Flow is one of the four buckets every transaction lands in. Matches
// the user-facing donuts on the Finance tab; "exclude" means the
// transaction is intentionally not counted (transfers between
// non-savings accounts, split parents, balance imports, etc.).
type Flow string

const (
	FlowIncome  Flow = "income"
	FlowSpent   Flow = "spent"
	FlowSaved   Flow = "saved"
	FlowExclude Flow = "exclude"
)

// ClassifyInput is the minimal projection of a transaction + its
// account that the classifier needs. Keeping it explicit makes the
// classifier trivially testable without spinning up SQLite.
type ClassifyInput struct {
	Amount            int64
	IsParent          bool
	TransferID        string
	Category          string // effective (override → upstream)
	Payee             string
	FlowOverride      string // 'income' | 'spent' | 'saved' | 'exclude' | ''
	AccountName       string
	AccountType       string // checking | savings | credit | investment | ...
	AccountOnBudget   bool
	IsSavingsDest     bool   // resolved (override → heuristic)
	IncludeInIncome   bool   // resolved (override → heuristic)
}

// Classify is the single source of truth for which donut a transaction
// rolls up into. Used by both the summary aggregator and the
// drilldown list endpoint so the two ALWAYS reconcile.
//
// Priority order (highest wins):
//  1. Explicit per-transaction flow_override
//  2. Structural exclusions (split parent, starting balance, payment cat)
//  3. Transfer logic — moves into savings count as Saved, other
//     transfers are zero-sum and excluded
//  4. Sign + account flags — positive on income-flagged accounts =
//     income; negative on on-budget accounts = spent
//  5. Everything else excluded
func Classify(in ClassifyInput) Flow {
	if f := normalizeFlowOverride(in.FlowOverride); f != "" {
		return f
	}
	if in.IsParent {
		return FlowExclude
	}
	cat := strings.ToLower(strings.TrimSpace(in.Category))
	payee := strings.ToLower(strings.TrimSpace(in.Payee))
	if cat == "starting balances" ||
		strings.HasPrefix(payee, "starting balance") ||
		strings.HasPrefix(payee, "initial balance") ||
		strings.HasPrefix(payee, "opening balance") {
		return FlowExclude
	}
	if cat == "credit card payment" || cat == "payment" ||
		strings.Contains(cat, "credit card payment") {
		return FlowExclude
	}

	isTransfer := strings.TrimSpace(in.TransferID) != ""

	switch {
	case in.Amount > 0:
		if in.IsSavingsDest {
			// Inflow into a savings destination. Balance-import rows
			// typically have an empty payee or one that matches the
			// account name — exclude those; everything else is a real
			// deposit (employer 401K contribution, transfer in,
			// brokerage deposit).
			if !isTransfer && (payee == "" ||
				payee == strings.ToLower(in.AccountName)) {
				return FlowExclude
			}
			return FlowSaved
		}
		if isTransfer {
			// Non-savings transfer — zero-sum, drop.
			return FlowExclude
		}
		if in.IncludeInIncome {
			return FlowIncome
		}
		return FlowExclude
	case in.Amount < 0:
		if isTransfer {
			return FlowExclude
		}
		if in.AccountOnBudget {
			return FlowSpent
		}
		return FlowExclude
	}
	return FlowExclude
}

func normalizeFlowOverride(s string) Flow {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "income":
		return FlowIncome
	case "spent":
		return FlowSpent
	case "saved":
		return FlowSaved
	case "exclude":
		return FlowExclude
	}
	return ""
}

// DefaultIncludeInIncome returns the heuristic default for whether an
// account participates in income aggregation. Credit cards, mortgage,
// and debt accounts default OUT (their +amounts are payment receipts,
// not income). Everything else defaults IN — the user can override
// from the iOS account flags sheet.
func DefaultIncludeInIncome(accountType, accountName string) bool {
	t := strings.ToLower(strings.TrimSpace(accountType))
	switch t {
	case "credit", "mortgage", "debt":
		return false
	}
	lower := strings.ToLower(accountName)
	if strings.Contains(lower, "credit card") ||
		strings.Contains(lower, "platinum") ||
		strings.Contains(lower, "sapphire") ||
		strings.Contains(lower, "apple card") ||
		strings.Contains(lower, " card") {
		return false
	}
	return true
}

// DefaultIsSavingsDestination uses account.type when present, falling
// back to the name heuristic. Type "savings" and "investment" default
// IN; everything else defaults to the name heuristic (which keeps the
// existing behavior for users whose Actual install doesn't expose
// type).
func DefaultIsSavingsDestination(accountType, accountName string) bool {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "savings", "investment":
		return true
	case "credit", "mortgage", "debt":
		return false
	}
	return isSavingsDestination(accountName)
}

package finance

import (
	"context"
	"database/sql"
	"strings"
)

// loadClassifiedMonth pulls every transaction for the month plus the
// minimal account context the classifier needs, runs Classify on each,
// and returns the rows with their resolved flow. Used by both the
// summary aggregator (sum by flow) and the drilldown list endpoint
// (filter by flow) so the two ALWAYS reconcile.
//
// monthLike is the SQL LIKE pattern, e.g. "2026-05-%".
type ClassifiedRow struct {
	ID               string
	AccountID        string
	AccountName      string
	Date             string
	Payee            string
	Category         string // effective (override → upstream)
	CategoryOriginal string
	AmountCents      int64
	Notes            string
	Flow             Flow
	// FlowOverrideRaw is the persisted column value (empty when NULL).
	// Exposed so handlers can pass it through to the DTO for the iOS
	// flow picker.
	FlowOverrideRaw string
}

func loadClassifiedMonth(ctx context.Context, db *sql.DB, monthLike string) ([]ClassifiedRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.actual_id, t.account_id, COALESCE(a.name,''), t.date,
		        COALESCE(t.payee,''),
		        COALESCE(NULLIF(t.category_user,''), COALESCE(t.category,'')) AS effective,
		        CASE WHEN NULLIF(t.category_user,'') IS NOT NULL
		             THEN COALESCE(t.category,'') ELSE '' END AS original,
		        t.amount_cents, COALESCE(t.notes,''),
		        t.transfer_id, t.is_parent,
		        COALESCE(t.flow_override,''),
		        COALESCE(a.type,''), a.on_budget,
		        a.is_savings_destination,
		        a.include_in_income
		   FROM fin_transactions t
		   LEFT JOIN fin_accounts a ON a.actual_id = t.account_id
		  WHERE t.date LIKE ?
		  ORDER BY t.date DESC, t.imported_at DESC`, monthLike)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ClassifiedRow, 0, 64)
	for rows.Next() {
		var r ClassifiedRow
		var transferID sql.NullString
		var isParent int
		var flowOverride, accType string
		var onBudget int
		var savingsOverride, incomeOverride sql.NullInt64
		if err := rows.Scan(&r.ID, &r.AccountID, &r.AccountName, &r.Date,
			&r.Payee, &r.Category, &r.CategoryOriginal,
			&r.AmountCents, &r.Notes,
			&transferID, &isParent, &flowOverride,
			&accType, &onBudget,
			&savingsOverride, &incomeOverride); err != nil {
			return nil, err
		}

		isSavings := resolveBoolFlag(savingsOverride,
			DefaultIsSavingsDestination(accType, r.AccountName))
		includeIncome := resolveBoolFlag(incomeOverride,
			DefaultIncludeInIncome(accType, r.AccountName))

		r.FlowOverrideRaw = flowOverride
		r.Flow = Classify(ClassifyInput{
			Amount:          r.AmountCents,
			IsParent:        isParent == 1,
			TransferID:      strings.TrimSpace(transferID.String),
			Category:        r.Category,
			Payee:           r.Payee,
			FlowOverride:    flowOverride,
			AccountName:     r.AccountName,
			AccountType:     accType,
			AccountOnBudget: onBudget == 1,
			IsSavingsDest:   isSavings,
			IncludeInIncome: includeIncome,
		})
		out = append(out, r)
	}
	return out, rows.Err()
}

func resolveBoolFlag(override sql.NullInt64, heuristic bool) bool {
	if override.Valid {
		return override.Int64 == 1
	}
	return heuristic
}

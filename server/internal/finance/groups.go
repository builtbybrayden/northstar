package finance

import "strings"

// Category group taxonomy. Five buckets the iOS Finance tab renders as
// collapsible headers. Users can override any individual category's group
// via PATCH /api/finance/budget-targets/:category.
const (
	GroupLiving   = "Living Expenses"
	GroupTrans    = "Transportation"
	GroupDining   = "Dining & Entertainment"
	GroupSavings  = "Savings & Income"
	GroupMisc     = "Miscellaneous"
)

// AllGroups is the ordered list the iOS app uses when rendering sections.
// Order is deliberate — fixed needs first, discretionary next, income last,
// catch-all at the bottom.
var AllGroups = []string{
	GroupLiving,
	GroupTrans,
	GroupDining,
	GroupSavings,
	GroupMisc,
}

// DefaultGroupFor returns the seed group for a category whose user-set
// group is empty. Heuristic on lowercase name keywords; falls back to
// Miscellaneous when nothing matches.
//
// Intentionally conservative — if the heuristic disagrees with what the
// user wants, the PATCH endpoint lets them re-bucket without code changes.
func DefaultGroupFor(category string) string {
	c := strings.ToLower(strings.TrimSpace(category))
	if c == "" {
		return GroupMisc
	}
	// Income / savings — handle these first since "Rental Income" would
	// otherwise fall through to misc.
	if containsAnyStr(c, "salary", "income", "reimburs", "transfer",
		"401k", "ira", "retirement", "savings", "investment", "dividend",
		"starting balance") {
		return GroupSavings
	}
	if containsAnyStr(c, "mortgage", "rent ", "rental",
		"utilities", "phone", "internet", "wifi",
		"home", "house",
		"groceries", "grocery",
		"medical", "health", "dental", "vision", "pharmacy",
		"insurance",
		"childcare", "daycare", "tuition", "education", "student loan",
		"pet care", "vet", "dog", "cat ") {
		return GroupLiving
	}
	if containsAnyStr(c, "gas", "fuel",
		"vehicle", "car ", "auto", "uber", "lyft", "taxi", "transit",
		"parking", "toll", "rideshare", "subway", "train", "airline") {
		return GroupTrans
	}
	if containsAnyStr(c, "restaurant", "dining", "bar ", "coffee",
		"entertainment", "subscription", "streaming", "concert",
		"travel", "vacation", "hotel", "lodging",
		"gift", "shopping", "online shopping", "amazon",
		"personal care", "salon", "barber", "spa", "gym", "fitness") {
		return GroupDining
	}
	// Anything else (Cash, Government Fees, P2P Transfers, Uncategorized) — Misc.
	return GroupMisc
}

func containsAnyStr(haystack string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

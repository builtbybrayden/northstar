// Mock data provider — returns realistic-looking Actual-shaped data so the
// rest of the stack works end-to-end without a real Actual server.
//
// Numbers are inspired by the user's actual category set + accounts but
// randomized day-to-day so the dashboard isn't static.

const today = () => new Date().toISOString().slice(0, 10);

function daysAgo(n) {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

export function mockAccounts() {
  return [
    { id: 'acc-chase-checking',   name: 'Chase Checking (8321)',        offbudget: false, closed: false, balance: 384217  },
    { id: 'acc-amex-checking',    name: 'Amex Checking (4823)',         offbudget: false, closed: false, balance: 218400  },
    { id: 'acc-amex-hysa',        name: 'Amex HYSA (4289)',             offbudget: false, closed: false, balance: 1842600 },
    { id: 'acc-meritrust',        name: 'Meritrust Membership (7837)',  offbudget: false, closed: false, balance: 12500   },
    { id: 'acc-apple-card',       name: 'Apple Card',                   offbudget: false, closed: false, balance: -28341  },
    { id: 'acc-amex-platinum',    name: 'Amex Platinum (2008)',         offbudget: false, closed: false, balance: -89412  },
    { id: 'acc-chase-sapphire',   name: 'Chase Sapphire (7499)',        offbudget: false, closed: false, balance: -41208  },
    { id: 'acc-chase-mortgage',   name: 'Chase Mortgage (1040)',        offbudget: true,  closed: false, balance: -24800000 },
    { id: 'acc-rh-individual',    name: 'Robinhood Individual',         offbudget: true,  closed: false, balance: 2840100 },
    { id: 'acc-rh-crypto',        name: 'Robinhood Crypto',             offbudget: true,  closed: false, balance: 412800  },
    { id: 'acc-401k',             name: 'American Express 401k',        offbudget: true,  closed: false, balance: 1554200 },
  ];
}

export function mockCategories() {
  return [
    { id: 'cat-groceries',     name: 'Groceries',     group_id: 'g-needs',   is_income: false },
    { id: 'cat-restaurants',   name: 'Restaurants',   group_id: 'g-wants',   is_income: false },
    { id: 'cat-gas',           name: 'Gas',           group_id: 'g-needs',   is_income: false },
    { id: 'cat-utilities',     name: 'Utilities',     group_id: 'g-needs',   is_income: false },
    { id: 'cat-mortgage',      name: 'Mortgage',      group_id: 'g-needs',   is_income: false },
    { id: 'cat-subscriptions', name: 'Subscriptions', group_id: 'g-wants',   is_income: false },
    { id: 'cat-travel',        name: 'Travel',        group_id: 'g-wants',   is_income: false },
    { id: 'cat-medical',       name: 'Medical',       group_id: 'g-needs',   is_income: false },
    { id: 'cat-gifts',         name: 'Gifts',         group_id: 'g-wants',   is_income: false },
    { id: 'cat-salary',        name: 'Salary',        group_id: 'g-income',  is_income: true  },
    { id: 'cat-transfer',      name: 'Transfer',      group_id: 'g-system',  is_income: false },
  ];
}

// Returns a deterministic-ish set of transactions for May-of-current-year
// matching the budget rings in the mockup.
export function mockTransactions() {
  const fixed = [
    // ── Restaurants (over budget at $612) ─────────────────────────────────
    { id: 'tx-rest-1',  account: 'acc-chase-sapphire', date: today(),       payee: 'Starbucks #4318',    category: 'cat-restaurants', amount:   -745, notes: '' },
    { id: 'tx-rest-2',  account: 'acc-apple-card',     date: daysAgo(2),    payee: 'DoorDash · Mei Wei', category: 'cat-restaurants', amount:  -3418, notes: '' },
    { id: 'tx-rest-3',  account: 'acc-amex-platinum',  date: daysAgo(4),    payee: 'Antico Pizza',       category: 'cat-restaurants', amount:  -5840, notes: '' },
    { id: 'tx-rest-4',  account: 'acc-amex-platinum',  date: daysAgo(6),    payee: 'Cooks & Soldiers',   category: 'cat-restaurants', amount: -14892, notes: 'with fiancée' },
    { id: 'tx-rest-5',  account: 'acc-chase-sapphire', date: daysAgo(7),    payee: 'Starbucks #4318',    category: 'cat-restaurants', amount:   -745, notes: '' },
    { id: 'tx-rest-6',  account: 'acc-apple-card',     date: daysAgo(9),    payee: 'Chipotle',           category: 'cat-restaurants', amount:  -1842, notes: '' },
    { id: 'tx-rest-7',  account: 'acc-apple-card',     date: daysAgo(11),   payee: 'DoorDash · Sushi',   category: 'cat-restaurants', amount:  -4221, notes: '' },
    { id: 'tx-rest-8',  account: 'acc-chase-sapphire', date: daysAgo(12),   payee: 'Starbucks #4318',    category: 'cat-restaurants', amount:   -745, notes: '' },
    { id: 'tx-rest-9',  account: 'acc-amex-platinum',  date: daysAgo(14),   payee: 'Taqueria del Sol',   category: 'cat-restaurants', amount:  -3812, notes: '' },
    { id: 'tx-rest-10', account: 'acc-apple-card',     date: daysAgo(16),   payee: 'DoorDash · Thai',    category: 'cat-restaurants', amount:  -2937, notes: '' },
    { id: 'tx-rest-11', account: 'acc-chase-sapphire', date: daysAgo(17),   payee: 'Local diner',        category: 'cat-restaurants', amount:  -3018, notes: '' },

    // ── Groceries ($427 of $700) ──────────────────────────────────────────
    { id: 'tx-groc-1', account: 'acc-apple-card',     date: daysAgo(1),    payee: 'Kroger',            category: 'cat-groceries',   amount:  -8412, notes: '' },
    { id: 'tx-groc-2', account: 'acc-amex-platinum',  date: daysAgo(5),    payee: 'Whole Foods',       category: 'cat-groceries',   amount: -11280, notes: '' },
    { id: 'tx-groc-3', account: 'acc-apple-card',     date: daysAgo(8),    payee: 'Trader Joes',       category: 'cat-groceries',   amount:  -6217, notes: '' },
    { id: 'tx-groc-4', account: 'acc-apple-card',     date: daysAgo(13),   payee: 'Kroger',            category: 'cat-groceries',   amount:  -9128, notes: '' },
    { id: 'tx-groc-5', account: 'acc-amex-platinum',  date: daysAgo(15),   payee: 'Whole Foods',       category: 'cat-groceries',   amount:  -7621, notes: '' },

    // ── Gas ($178 of $260) ────────────────────────────────────────────────
    { id: 'tx-gas-1',  account: 'acc-apple-card',     date: daysAgo(3),    payee: 'Shell',             category: 'cat-gas',         amount:  -4218, notes: '' },
    { id: 'tx-gas-2',  account: 'acc-apple-card',     date: daysAgo(10),   payee: 'Chevron',           category: 'cat-gas',         amount:  -4892, notes: '' },
    { id: 'tx-gas-3',  account: 'acc-apple-card',     date: daysAgo(16),   payee: 'QuikTrip',          category: 'cat-gas',         amount:  -8721, notes: '' },

    // ── Mortgage (paid in full, $2005) ────────────────────────────────────
    { id: 'tx-mtg-1',  account: 'acc-amex-hysa',      date: '2026-05-01',  payee: 'Chase Mortgage',    category: 'cat-mortgage',    amount: -200472, notes: 'autopay' },

    // ── Utilities ($143 of $220) ──────────────────────────────────────────
    { id: 'tx-util-1', account: 'acc-chase-checking', date: daysAgo(4),    payee: 'Georgia Power',     category: 'cat-utilities',   amount:  -8421, notes: '' },
    { id: 'tx-util-2', account: 'acc-chase-checking', date: daysAgo(7),    payee: 'AT&T Fiber',        category: 'cat-utilities',   amount:  -6017, notes: '' },

    // ── Subscriptions ($93 of $90, slightly over) ─────────────────────────
    { id: 'tx-sub-1',  account: 'acc-apple-card',     date: daysAgo(2),    payee: 'Anthropic',         category: 'cat-subscriptions', amount: -2000, notes: '' },
    { id: 'tx-sub-2',  account: 'acc-apple-card',     date: daysAgo(8),    payee: 'Netflix',           category: 'cat-subscriptions', amount: -1599, notes: '' },
    { id: 'tx-sub-3',  account: 'acc-apple-card',     date: daysAgo(11),   payee: 'Spotify',           category: 'cat-subscriptions', amount: -1099, notes: '' },
    { id: 'tx-sub-4',  account: 'acc-apple-card',     date: daysAgo(14),   payee: 'iCloud+',           category: 'cat-subscriptions', amount:  -999, notes: '' },
    { id: 'tx-sub-5',  account: 'acc-apple-card',     date: daysAgo(17),   payee: 'GitHub Pro',        category: 'cat-subscriptions', amount:  -400, notes: '' },
    { id: 'tx-sub-6',  account: 'acc-apple-card',     date: daysAgo(18),   payee: 'NYTimes',           category: 'cat-subscriptions', amount: -2599, notes: '' },

    // ── Salary (income) ───────────────────────────────────────────────────
    { id: 'tx-pay-1',  account: 'acc-chase-checking', date: '2026-05-01',  payee: 'American Express PAYROLL', category: 'cat-salary', amount: 113235, notes: '' },
    { id: 'tx-pay-2',  account: 'acc-amex-hysa',      date: '2026-05-01',  payee: 'AMERICAN EXPRESS PAYROLL', category: 'cat-salary', amount:  60000, notes: '' },
    { id: 'tx-pay-3',  account: 'acc-amex-checking',  date: '2026-05-01',  payee: 'Online Transfer',          category: 'cat-salary', amount:  89500, notes: '' },
  ];
  return fixed;
}

export function mockBudgets() {
  return {
    month: new Date().toISOString().slice(0, 7),
    categories: [
      { id: 'cat-groceries',     budgeted: 70000  },
      { id: 'cat-restaurants',   budgeted: 50000  },
      { id: 'cat-gas',           budgeted: 26000  },
      { id: 'cat-utilities',     budgeted: 22000  },
      { id: 'cat-mortgage',      budgeted: 200500 },
      { id: 'cat-subscriptions', budgeted: 9000   },
      { id: 'cat-travel',        budgeted: 30000  },
      { id: 'cat-medical',       budgeted: 15000  },
      { id: 'cat-gifts',         budgeted: 10000  },
    ]
  };
}

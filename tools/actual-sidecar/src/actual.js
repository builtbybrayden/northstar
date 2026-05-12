// Real Actual mode — wraps @actual-app/api. Lazy-loaded so mock mode
// works without the dependency installed.

let api = null;
let initialized = false;

async function lazyLoad() {
  if (!api) {
    api = await import('@actual-app/api');
  }
  return api;
}

export async function init({ serverURL, password, syncId, dataDir }) {
  const a = await lazyLoad();
  await a.init({
    serverURL,
    password,
    dataDir: dataDir || './actual-data'
  });
  await a.downloadBudget(syncId, { password });
  initialized = true;
}

function requireInit() {
  if (!initialized) throw new Error('Actual not initialized. POST /init first.');
}

export async function accounts() {
  requireInit();
  const a = await lazyLoad();
  const accs = await a.getAccounts();
  // Balance is calculated separately
  const out = [];
  for (const acc of accs) {
    const balance = await a.getAccountBalance(acc.id);
    out.push({
      id: acc.id,
      name: acc.name,
      offbudget: !!acc.offbudget,
      closed: !!acc.closed,
      balance: balance ?? 0,
    });
  }
  return out;
}

export async function categories() {
  requireInit();
  const a = await lazyLoad();
  const cats = await a.getCategories();
  return cats.map(c => ({
    id: c.id,
    name: c.name,
    group_id: c.group_id,
    is_income: !!c.is_income,
  }));
}

export async function transactions({ since } = {}) {
  requireInit();
  const a = await lazyLoad();
  const accs = await a.getAccounts();
  const sinceStr = since || '1970-01-01';
  const today = new Date().toISOString().slice(0, 10);

  const out = [];
  for (const acc of accs) {
    const rows = await a.getTransactions(acc.id, sinceStr, today);
    for (const r of rows) {
      out.push({
        id: r.id,
        account: acc.id,
        date: r.date,
        payee: r.payee_name || r.payee || '',
        category: r.category || null,
        amount: r.amount,         // cents (negative = outflow)
        notes: r.notes || '',
      });
    }
  }
  return out;
}

export async function budgets({ month } = {}) {
  requireInit();
  const a = await lazyLoad();
  const m = month || new Date().toISOString().slice(0, 7);
  const monthData = await a.getBudgetMonth(m);
  const categories = [];
  for (const grp of monthData.categoryGroups || []) {
    for (const c of grp.categories || []) {
      categories.push({ id: c.id, budgeted: c.budgeted || 0 });
    }
  }
  return { month: m, categories };
}

export async function shutdown() {
  if (!initialized) return;
  const a = await lazyLoad();
  await a.shutdown();
  initialized = false;
}

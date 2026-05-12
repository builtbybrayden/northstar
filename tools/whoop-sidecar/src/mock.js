// Mock biometric data — returns realistic-looking WHOOP-shaped rows so the
// stack works end-to-end without a real WHOOP account.
//
// Anchors on today's date: today = green recovery (84%), yesterday = mid (62%),
// 3 days ago = dip into red (28%, simulates a poor sleep day) so the detector
// has something to flag.

function daysAgo(n) {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

// 14-day rolling window. Higher index = older.
// Each entry tagged with what makes it interesting for testing.
const RECOVERY = [
  { day:  0, score: 84, hrv:  78, rhr: 51 },  // today — green
  { day:  1, score: 62, hrv:  61, rhr: 54 },  // yesterday — yellow
  { day:  2, score: 71, hrv:  68, rhr: 52 },
  { day:  3, score: 28, hrv:  41, rhr: 60 },  // big dip — should fire health_insight
  { day:  4, score: 81, hrv:  74, rhr: 51 },
  { day:  5, score: 79, hrv:  72, rhr: 52 },
  { day:  6, score: 65, hrv:  64, rhr: 54 },
  { day:  7, score: 72, hrv:  69, rhr: 53 },
  { day:  8, score: 88, hrv:  82, rhr: 49 },
  { day:  9, score: 76, hrv:  70, rhr: 52 },
  { day: 10, score: 81, hrv:  75, rhr: 51 },
  { day: 11, score: 69, hrv:  66, rhr: 53 },
  { day: 12, score: 84, hrv:  78, rhr: 51 },
  { day: 13, score: 77, hrv:  71, rhr: 52 },
];

export function mockRecovery() {
  return RECOVERY.map(r => ({
    date: daysAgo(r.day),
    score: r.score,
    hrv_ms: r.hrv,
    rhr: r.rhr,
  }));
}

export function mockSleep() {
  return RECOVERY.map(r => {
    // Loose correlation: better recovery ≈ longer/better sleep
    const score = Math.max(55, Math.min(95, r.score + 4));
    const duration = score >= 80 ? 462 + (r.day % 3) * 5
                                 : (score >= 65 ? 415 : 360);
    const debt = score >= 80 ? 0 : (score >= 65 ? 30 : 70);
    return {
      date: daysAgo(r.day),
      duration_min: duration,
      score,
      debt_min: debt,
    };
  });
}

export function mockStrain() {
  return RECOVERY.map((r, i) => {
    // Higher recovery = higher strain achieved (the user pushed)
    const score = r.score >= 75 ? 16 + (i % 4) * 0.5
                : r.score >= 60 ? 13 + (i % 3) * 0.4
                : 9 + (i % 3) * 0.3;
    return {
      date: daysAgo(r.day),
      score: Number(score.toFixed(1)),
      avg_hr: 92 + Math.floor(r.score / 10),
      max_hr: 158 + Math.floor(r.score / 5),
    };
  });
}

export function mockProfile() {
  return {
    user_id: 'mock-user',
    first_name: 'Brayden',
    height_m: 1.85,
    weight_kg: 82,
  };
}

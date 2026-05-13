-- +goose Up
-- Per-transaction user-applied category override.
-- The sync worker pulls from Actual and writes `category` (the upstream value).
-- The Northstar iOS app can re-categorize a single transaction (Groceries →
-- Restaurants, for example) without round-tripping to Actual. When
-- `category_user` is non-null, callers should use COALESCE(category_user,
-- category) for everything that consumes a category (summary aggregation,
-- detector logic, AI tools, list views).
--
-- The sync worker MUST NOT touch `category_user`. Upstream re-categorization
-- in Actual will update `category` but leave the user's local override intact.

ALTER TABLE fin_transactions ADD COLUMN category_user TEXT;

-- +goose Down
ALTER TABLE fin_transactions DROP COLUMN category_user;

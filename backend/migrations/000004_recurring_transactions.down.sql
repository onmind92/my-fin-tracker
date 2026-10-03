DROP INDEX IF EXISTS transactions_recurring_schedule_unique;
ALTER TABLE transactions DROP COLUMN IF EXISTS scheduled_for;
ALTER TABLE transactions DROP COLUMN IF EXISTS recurring_transaction_id;
DROP TABLE IF EXISTS recurring_transactions;


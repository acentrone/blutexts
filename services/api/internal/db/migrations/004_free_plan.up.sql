-- Allow 'free' plan for testing mode
-- 004_free_plan.up.sql

ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_plan_check;
ALTER TABLE accounts ADD CONSTRAINT accounts_plan_check
    CHECK (plan IN ('pending', 'free', 'monthly', 'annual'));

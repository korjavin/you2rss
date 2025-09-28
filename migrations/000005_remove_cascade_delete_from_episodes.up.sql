
ALTER TABLE episodes DROP CONSTRAINT episodes_subscription_id_fkey, ADD CONSTRAINT episodes_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL;

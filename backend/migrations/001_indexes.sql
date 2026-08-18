-- Indexes required for Round 2 (search), Round 4 (duplicates), Round 5 (4-table join under load).
-- The dump ships with zero indexes beyond primary keys, so every JOIN/lookup below
-- would otherwise be a full sequential scan across 15M/3M/2.4M/2M rows.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Round 2: exact-match search on email / phone
CREATE INDEX IF NOT EXISTS idx_ws_user_email_lower ON ws_user (lower(user_email));
CREATE INDEX IF NOT EXISTS idx_ws_user_msisdn ON ws_user (msisdn);

-- Round 2: fuzzy name search (trigram similarity + ILIKE)
CREATE INDEX IF NOT EXISTS idx_ws_user_full_name_trgm ON ws_user USING gin (full_name gin_trgm_ops);

-- Round 5: 4-table JOIN (user -> orders -> transactions, user -> activity) under 100 concurrent requests
CREATE INDEX IF NOT EXISTS idx_ws_orders_user_id ON ws_orders (user_id);
CREATE INDEX IF NOT EXISTS idx_ws_transactions_order_id ON ws_transactions (order_id);
CREATE INDEX IF NOT EXISTS idx_ws_user_activity_user_id ON ws_user_activity (user_id);

-- Round 4: IP-based duplicate account clustering
CREATE INDEX IF NOT EXISTS idx_ws_user_activity_ip ON ws_user_activity (ip_address);

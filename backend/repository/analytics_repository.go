package repository

import (
	"time"

	"gorm.io/gorm"
)

type QualityMetrics struct {
	TotalRecords        int64
	EmailPresent        int64
	EmailMissing        int64
	EmailUnique         int64
	EmailDuplicate      int64
	EmailInvalid        int64
	PhonePresent        int64
	PhoneMissing        int64
	PhoneUnique         int64
	PhoneDuplicate      int64
	PhoneMalformed      int64
	BirthDatePresent    int64
	BirthDateMissing    int64
	BirthDateInvalid    int64
	BirthDateImpossible int64
	BirthDateFuture     int64
	HobbiesNull         int64
	HobbiesSpecialChars int64
	StatusNeg1          int64
	Status0             int64
	Status1             int64
}

type IPDuplicateGroup struct {
	IPAddress     string
	UserCount     int64
	UserIDsJSON   string
	UserNamesJSON string
	FirstActivity time.Time
	LastActivity  time.Time
}

type DuplicatePair struct {
	ID1        int64
	ID2        int64
	Similarity float64
	MatchType  string
}

type UserProfileResult struct {
	UserID                 int64
	FullName               string
	UserEmail              *string
	Msisdn                 *string
	OrderCount             int64
	TotalTransactionAmount float64
	ActivityCount          int64
	LastActivity           *time.Time
}

type AnalyticsRepository interface {
	QualityMetrics() (*QualityMetrics, error)
	SampleInvalidEmails(limit int) ([]string, error)
	SampleMalformedPhones(limit int) ([]string, error)
	DuplicateGroupsByIP(limit int) ([]IPDuplicateGroup, error)
	ExactDuplicatePairs(limit int) ([]DuplicatePair, error)
	DuplicatesForUser(userID uint64, limit int) ([]DuplicatePair, error)
	UserProfile(userID uint64) (*UserProfileResult, error)
}

type analyticsRepository struct {
	db *gorm.DB
}

func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

// QualityMetrics computes every metric in a single sequential pass over ws_user
// (FILTER-based conditional aggregation) instead of one query per metric.
//
// birth_date has no way to be genuinely "unparseable" (it's a native `date`
// column, Postgres would reject bad literals at import time), so
// invalid/impossible/future are heuristics: impossible = sentinel-looking
// years (<=1 or >=9999), future = after today, invalid = the broader bucket
// of implausible-but-not-sentinel ages (>120 years old) unioned with the two above.
// bigWorkMemDB raises work_mem for this session only: COUNT(DISTINCT ...) and
// GROUP BY over the full 15M-row table need to hash/sort far more data than
// the default 64MB work_mem, and without room it spills to disk and the
// query blows past any reasonable timeout.
// Also lifts statement_timeout for this transaction only: these queries run
// from a background refresh goroutine (never on the request path), so they
// can safely take longer than the connection-wide interactive timeout.
// Raised from 120s to 280s after observing it hit the timeout repeatedly
// in production under concurrent load (e.g. while a k6 run was also hammering
// every other endpoint) -- the query still finishes, just slower under I/O
// contention on this VPS, and the goroutine loop is already sequential
// (single ticker, one refresh() at a time), so a longer timeout here just
// means "give the one background attempt a real chance to land" rather than
// looping forever on 120s cancellations and never populating the cache.
func bigWorkMemDB(db *gorm.DB) *gorm.DB {
	tx := db.Begin()
	tx.Exec("SET LOCAL work_mem = '512MB'")
	tx.Exec("SET LOCAL statement_timeout = '280s'")
	return tx
}

// QualityMetrics splits the work by cost instead of one query over
// everything:
//
//  1. A full sequential scan over the cheap, small, never-toasted columns
//     (user_email varchar(512), msisdn varchar(20), birth_date, status).
//     A plain COUNT(*) over this table consistently takes ~2s -- proof this
//     table's *sequential* access pattern is fine.
//  2. hobbies/about_me are large text columns that spend a lot of their
//     values out-of-line in TOAST storage; evaluating a regex against them
//     for all 15M rows means ~15M extra random reads to fetch TOAST chunks,
//     which is what actually made the original single-query version take
//     120s+ without finishing (confirmed live via pg_stat_activity: wait
//     event IO/DataFileRead, not CPU). So hobbies stats are estimated from a
//     1% TABLESAMPLE and scaled up instead of touched on every row.
//  3. email/phone "unique" counts use Postgres's own planner statistics
//     (pg_stats.n_distinct, populated by ANALYZE) instead of a live
//     COUNT(DISTINCT ...), which would force a 15M-row sort/hash of every
//     email string just to answer a dashboard estimate.
func (r *analyticsRepository) QualityMetrics() (*QualityMetrics, error) {
	// This runs from the same background refresh goroutine as
	// ExactDuplicatePairs (never on the request path), but was previously
	// left on the connection's default 20s statement_timeout -- and the
	// count(*) FILTER (...) scan below was measured taking ~18-20s on its
	// own, meaning it sat right at the edge of getting cancelled and
	// intermittently never populated the cache. bigWorkMemDB gives it the
	// same 280s runway as the duplicates refresh.
	tx := bigWorkMemDB(r.db)
	defer tx.Rollback()

	var m QualityMetrics
	err := tx.Raw(`
		SELECT
			count(*) AS total_records,
			count(*) FILTER (WHERE user_email IS NOT NULL AND user_email <> '') AS email_present,
			count(*) FILTER (WHERE user_email IS NULL OR user_email = '') AS email_missing,
			count(*) FILTER (WHERE user_email IS NOT NULL AND user_email <> '' AND user_email !~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$') AS email_invalid,
			count(*) FILTER (WHERE msisdn IS NOT NULL AND msisdn <> '') AS phone_present,
			count(*) FILTER (WHERE msisdn IS NULL OR msisdn = '') AS phone_missing,
			count(*) FILTER (WHERE msisdn IS NOT NULL AND msisdn <> '' AND (length(msisdn) < 8 OR msisdn !~ '^[0-9+]+$')) AS phone_malformed,
			count(birth_date) AS birth_date_present,
			count(*) - count(birth_date) AS birth_date_missing,
			count(*) FILTER (WHERE birth_date > CURRENT_DATE) AS birth_date_future,
			count(*) FILTER (WHERE birth_date IS NOT NULL AND (extract(year FROM birth_date) <= 1 OR extract(year FROM birth_date) >= 9999)) AS birth_date_impossible,
			count(*) FILTER (WHERE birth_date IS NOT NULL AND (birth_date > CURRENT_DATE OR extract(year FROM birth_date) <= 1 OR extract(year FROM birth_date) >= 9999 OR extract(year FROM age(birth_date)) > 120)) AS birth_date_invalid,
			count(*) FILTER (WHERE status = -1) AS status_neg1,
			count(*) FILTER (WHERE status = 0) AS status_0,
			count(*) FILTER (WHERE status = 1) AS status_1
		FROM ws_user
	`).Row().Scan(
		&m.TotalRecords, &m.EmailPresent, &m.EmailMissing, &m.EmailInvalid,
		&m.PhonePresent, &m.PhoneMissing, &m.PhoneMalformed,
		&m.BirthDatePresent, &m.BirthDateMissing, &m.BirthDateFuture, &m.BirthDateImpossible, &m.BirthDateInvalid,
		&m.StatusNeg1, &m.Status0, &m.Status1,
	)
	if err != nil {
		return nil, err
	}

	m.EmailUnique = r.estimateDistinct("ws_user", "user_email", m.EmailPresent)
	m.PhoneUnique = r.estimateDistinct("ws_user", "msisdn", m.PhonePresent)
	m.EmailDuplicate = max0(m.EmailPresent - m.EmailUnique)
	m.PhoneDuplicate = max0(m.PhonePresent - m.PhoneUnique)

	var sampleTotal, sampleHobbiesNull, sampleHobbiesSpecial int64
	err = tx.Raw(`
		SELECT count(*),
		       count(*) FILTER (WHERE hobbies IS NULL),
		       count(*) FILTER (WHERE hobbies ~ '[^\x00-\x7F]')
		FROM ws_user TABLESAMPLE SYSTEM (1)
	`).Row().Scan(&sampleTotal, &sampleHobbiesNull, &sampleHobbiesSpecial)
	if err != nil {
		return nil, err
	}
	if sampleTotal > 0 {
		scale := float64(m.TotalRecords) / float64(sampleTotal)
		m.HobbiesNull = int64(float64(sampleHobbiesNull)*scale + 0.5)
		m.HobbiesSpecialChars = int64(float64(sampleHobbiesSpecial)*scale + 0.5)
	}

	return &m, nil
}

// estimateDistinct reads Postgres's own ANALYZE-computed cardinality
// estimate (pg_stats.n_distinct) instead of running a live COUNT(DISTINCT).
// n_distinct >= 0 is an absolute estimate; negative means "-n_distinct * row
// count" (a ratio, used when most values are unique).
func (r *analyticsRepository) estimateDistinct(table, column string, present int64) int64 {
	var nDistinct float64
	err := r.db.Raw(`SELECT n_distinct FROM pg_stats WHERE tablename = ? AND attname = ?`, table, column).Scan(&nDistinct).Error
	if err != nil || nDistinct == 0 {
		return present
	}
	if nDistinct < 0 {
		return int64(-nDistinct*float64(present) + 0.5)
	}
	if int64(nDistinct) > present {
		return present
	}
	return int64(nDistinct)
}

func max0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func (r *analyticsRepository) SampleInvalidEmails(limit int) ([]string, error) {
	var out []string
	err := r.db.Raw(`
		SELECT user_email FROM ws_user
		WHERE user_email IS NOT NULL AND user_email <> ''
		  AND user_email !~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'
		LIMIT ?
	`, limit).Scan(&out).Error
	return out, err
}

func (r *analyticsRepository) SampleMalformedPhones(limit int) ([]string, error) {
	var out []string
	err := r.db.Raw(`
		SELECT msisdn FROM ws_user
		WHERE msisdn IS NOT NULL AND msisdn <> ''
		  AND (length(msisdn) < 8 OR msisdn !~ '^[0-9+]+$')
		LIMIT ?
	`, limit).Scan(&out).Error
	return out, err
}

func (r *analyticsRepository) DuplicateGroupsByIP(limit int) ([]IPDuplicateGroup, error) {
	var out []IPDuplicateGroup
	// user_ids/user_names are serialized to JSON text in SQL: scanning a native
	// Postgres array into a Go slice via GORM's generic Raw().Scan() silently
	// comes back nil (no pq.Array-style wrapper in play), so we unmarshal client-side.
	err := r.db.Raw(`
		WITH grouped AS (
			SELECT ip_address,
			       count(DISTINCT user_id) AS user_count,
			       array_agg(DISTINCT user_id ORDER BY user_id) AS user_ids,
			       min(activity_timestamp) AS first_activity,
			       max(activity_timestamp) AS last_activity
			FROM ws_user_activity
			WHERE ip_address IS NOT NULL AND ip_address <> ''
			GROUP BY ip_address
			HAVING count(DISTINCT user_id) > 1
			ORDER BY count(DISTINCT user_id) DESC
			LIMIT ?
		)
		SELECT g.ip_address, g.user_count,
		       to_json(g.user_ids)::text AS user_ids_json,
		       g.first_activity, g.last_activity,
		       to_json(array_agg(u.full_name ORDER BY u.user_id))::text AS user_names_json
		FROM grouped g
		JOIN ws_user u ON u.user_id = ANY(g.user_ids)
		GROUP BY g.ip_address, g.user_count, g.user_ids, g.first_activity, g.last_activity
		ORDER BY g.user_count DESC
	`, limit).Scan(&out).Error
	return out, err
}

func (r *analyticsRepository) ExactDuplicatePairs(limit int) ([]DuplicatePair, error) {
	var out []DuplicatePair
	tx := bigWorkMemDB(r.db)
	defer tx.Rollback()
	err := tx.Raw(`
		WITH email_groups AS (
			SELECT array_agg(user_id ORDER BY user_id) AS ids
			FROM ws_user
			WHERE user_email IS NOT NULL AND user_email <> ''
			GROUP BY lower(user_email)
			HAVING count(*) > 1
		),
		phone_groups AS (
			SELECT array_agg(user_id ORDER BY user_id) AS ids
			FROM ws_user
			WHERE msisdn IS NOT NULL AND msisdn <> ''
			GROUP BY msisdn
			HAVING count(*) > 1
		)
		SELECT g.ids[s.i] AS id1, g.ids[s.i+1] AS id2, 1.0::float8 AS similarity, 'email'::text AS match_type
		FROM email_groups g CROSS JOIN LATERAL generate_series(1, array_length(g.ids,1)-1) AS s(i)
		UNION ALL
		SELECT g.ids[s.i] AS id1, g.ids[s.i+1] AS id2, 1.0::float8 AS similarity, 'phone'::text AS match_type
		FROM phone_groups g CROSS JOIN LATERAL generate_series(1, array_length(g.ids,1)-1) AS s(i)
		LIMIT ?
	`, limit).Scan(&out).Error
	return out, err
}

func (r *analyticsRepository) DuplicatesForUser(userID uint64, limit int) ([]DuplicatePair, error) {
	var out []DuplicatePair
	err := r.db.Raw(`
		SELECT u1.user_id AS id1, u2.user_id AS id2, 1.0::float8 AS similarity, 'email'::text AS match_type
		FROM ws_user u1
		JOIN ws_user u2 ON lower(u2.user_email) = lower(u1.user_email) AND u2.user_id <> u1.user_id
		WHERE u1.user_id = ? AND u1.user_email IS NOT NULL AND u1.user_email <> ''
		UNION
		SELECT u1.user_id, u2.user_id, 1.0::float8, 'phone'::text
		FROM ws_user u1
		JOIN ws_user u2 ON u2.msisdn = u1.msisdn AND u2.user_id <> u1.user_id
		WHERE u1.user_id = ? AND u1.msisdn IS NOT NULL AND u1.msisdn <> ''
		LIMIT ?
	`, userID, userID, limit).Scan(&out).Error
	return out, err
}

func (r *analyticsRepository) UserProfile(userID uint64) (*UserProfileResult, error) {
	var out UserProfileResult
	err := r.db.Raw(`
		SELECT
			u.user_id,
			u.full_name,
			u.user_email,
			u.msisdn,
			COALESCE(o.order_count, 0) AS order_count,
			COALESCE(tx.total_amount, 0) AS total_transaction_amount,
			COALESCE(a.activity_count, 0) AS activity_count,
			a.last_activity
		FROM ws_user u
		LEFT JOIN (
			SELECT user_id, count(*) AS order_count
			FROM ws_orders WHERE user_id = ? GROUP BY user_id
		) o ON o.user_id = u.user_id
		LEFT JOIN (
			SELECT o2.user_id, sum(t.transaction_amount) AS total_amount
			FROM ws_orders o2
			JOIN ws_transactions t ON t.order_id = o2.order_id
			WHERE o2.user_id = ?
			GROUP BY o2.user_id
		) tx ON tx.user_id = u.user_id
		LEFT JOIN (
			SELECT user_id, count(*) AS activity_count, max(activity_timestamp) AS last_activity
			FROM ws_user_activity WHERE user_id = ? GROUP BY user_id
		) a ON a.user_id = u.user_id
		WHERE u.user_id = ?
	`, userID, userID, userID, userID).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	if out.UserID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &out, nil
}

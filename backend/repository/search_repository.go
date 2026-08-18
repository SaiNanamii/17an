package repository

import "gorm.io/gorm"

type SearchResult struct {
	UserID    int64
	FullName  string
	UserEmail *string
	Msisdn    *string
	Status    int16
	CreatedAt *string
}

type SearchRepository interface {
	SearchByEmail(email string, limit, offset int) ([]SearchResult, int64, error)
	SearchByPhone(phone string, limit, offset int) ([]SearchResult, int64, error)
	SearchByUserID(userID uint64) ([]SearchResult, int64, error)
	SearchByName(name string, limit, offset int) ([]SearchResult, int64, error)
}

type searchRepository struct {
	db *gorm.DB
}

func NewSearchRepository(db *gorm.DB) SearchRepository {
	return &searchRepository{db: db}
}

const searchSelectCols = `user_id, full_name, user_email, msisdn, status, to_char(create_time, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at`

func (r *searchRepository) SearchByEmail(email string, limit, offset int) ([]SearchResult, int64, error) {
	var out []SearchResult
	var total int64
	if err := r.db.Raw(`SELECT count(*) FROM ws_user WHERE lower(user_email) = lower(?)`, email).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Raw(`SELECT `+searchSelectCols+` FROM ws_user WHERE lower(user_email) = lower(?) ORDER BY user_id LIMIT ? OFFSET ?`, email, limit, offset).Scan(&out).Error
	return out, total, err
}

func (r *searchRepository) SearchByPhone(phone string, limit, offset int) ([]SearchResult, int64, error) {
	var out []SearchResult
	var total int64
	if err := r.db.Raw(`SELECT count(*) FROM ws_user WHERE msisdn = ?`, phone).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Raw(`SELECT `+searchSelectCols+` FROM ws_user WHERE msisdn = ? ORDER BY user_id LIMIT ? OFFSET ?`, phone, limit, offset).Scan(&out).Error
	return out, total, err
}

func (r *searchRepository) SearchByUserID(userID uint64) ([]SearchResult, int64, error) {
	var out []SearchResult
	err := r.db.Raw(`SELECT `+searchSelectCols+` FROM ws_user WHERE user_id = ?`, userID).Scan(&out).Error
	return out, int64(len(out)), err
}

// SearchByName uses the `%` trigram operator (not the similarity() function
// directly) because only `%`/ILIKE-on-pattern are backed by the GIN trgm
// index (idx_ws_user_full_name_trgm) -- calling similarity() as a plain
// WHERE-clause predicate forces a sequential scan of all 15M rows.
//
// similarity_threshold is set to 0.45 rather than pg_trgm's 0.3 default:
// at 0.3, a short query like "john" matches ~120K rows via the `%` operator
// alone (measured via EXPLAIN ANALYZE against the live dataset, ~1.6s just
// to count), which is loose enough to blow well past the <500ms budget
// under concurrent load. 0.45 cuts that to a low-tens-of-thousands range
// while still tolerating minor typos, and count+rows are fetched in one
// scan (via a window function) instead of two, since the WHERE predicate
// was previously evaluated twice per request.
func (r *searchRepository) SearchByName(name string, limit, offset int) ([]SearchResult, int64, error) {
	var out []struct {
		SearchResult
		Total int64 `gorm:"column:total_count"`
	}
	tx := r.db.Begin()
	defer tx.Rollback()
	tx.Exec("SET LOCAL pg_trgm.similarity_threshold = 0.45")

	err := tx.Raw(`
		SELECT `+searchSelectCols+`,
		       GREATEST(similarity(full_name, ?), CASE WHEN full_name ILIKE '%' || ? || '%' THEN 0.5 ELSE 0 END) AS rank,
		       count(*) OVER() AS total_count
		FROM ws_user
		WHERE full_name ILIKE '%' || ? || '%' OR full_name % ?
		ORDER BY rank DESC, user_id
		LIMIT ? OFFSET ?
	`, name, name, name, name, limit, offset).Scan(&out).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResult, 0, len(out))
	var total int64
	if len(out) > 0 {
		total = out[0].Total
	}
	for _, row := range out {
		results = append(results, row.SearchResult)
	}
	return results, total, nil
}

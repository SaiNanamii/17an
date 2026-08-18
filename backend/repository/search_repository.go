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

// SearchByName uses pg_trgm's word_similarity (the `%>` operator, backed by
// the same GIN trgm index, idx_ws_user_full_name_trgm) instead of whole-
// string similarity()/ILIKE. word_similarity scores the best-matching WORD
// within full_name against the query, so "john" correctly scores high
// against "John Doe Wijaya" even though whole-string trigram similarity of
// a 4-char query against a long multi-word name is naturally low (diluted
// by the other words) -- the previous implementation compensated for that
// with a separate ILIKE '%...%' OR'd in, which doubled the index scans
// (BitmapOr) and left ~14.6K rows needing a heap recheck.
//
// word_similarity_threshold=0.65 was chosen by measuring EXPLAIN ANALYZE
// against the live dataset: it matches a comparable row count to the old
// combined query (~6.6K rows for "john" vs ~6.8K before) with only ~11 rows
// removed by recheck (vs ~14.6K), a single index condition instead of two,
// and total execution time ~370ms unloaded (vs ~340ms-2.2s depending on
// which version/threshold was live) -- same or better speed, and more
// semantically correct results.
func (r *searchRepository) SearchByName(name string, limit, offset int) ([]SearchResult, int64, error) {
	var out []struct {
		SearchResult
		Total int64 `gorm:"column:total_count"`
	}
	tx := r.db.Begin()
	defer tx.Rollback()
	tx.Exec("SET LOCAL pg_trgm.word_similarity_threshold = 0.65")

	err := tx.Raw(`
		SELECT `+searchSelectCols+`,
		       word_similarity(?, full_name) AS rank,
		       count(*) OVER() AS total_count
		FROM ws_user
		WHERE full_name %> ?
		ORDER BY rank DESC, user_id
		LIMIT ? OFFSET ?
	`, name, name, limit, offset).Scan(&out).Error
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

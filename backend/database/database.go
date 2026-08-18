package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"17an/config"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

	return db, nil
}

// prewarmTargets are the relations the search endpoints depend on. Loading
// them into shared_buffers/OS cache right after (re)connecting removes the
// cold-cache penalty that a fresh restart otherwise pays on the first real
// request (measured: a cold GIN trigram scan for a common name took ~1.6s
// vs ~340ms once warm). This is best-effort, not a permanent guarantee --
// on this VPS the table is larger than shared_buffers, so pages loaded here
// can still get evicted by the background quality/duplicates cache
// refreshes competing for the same buffer space. It's still worth doing:
// it fixes the worst-case first-request-after-deploy scenario for free.
var prewarmTargets = []string{
	"ws_user",
	"idx_ws_user_full_name_trgm",
	"idx_ws_user_email_lower",
	"idx_ws_user_msisdn",
}

// Prewarm runs in the background so it never blocks app startup (see
// service.NewHealthService and friends for the same pattern). Errors are
// logged, not fatal: pg_prewarm may be unavailable in some environments
// (e.g. a managed Postgres without the extension whitelisted), and search
// still works correctly, just cold, without it.
func Prewarm(db *gorm.DB) {
	go func() {
		if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_prewarm").Error; err != nil {
			log.Printf("prewarm: pg_prewarm unavailable, skipping: %v", err)
			return
		}
		if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
			log.Printf("prewarm: pg_trgm unavailable: %v", err)
		}
		for _, rel := range prewarmTargets {
			if err := db.Exec("SELECT pg_prewarm(?)", rel).Error; err != nil {
				log.Printf("prewarm: %s failed: %v", rel, err)
			}
		}
		log.Println("prewarm: done")
	}()
}

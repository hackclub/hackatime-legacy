package migrations

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/hackclub/hackatime/config"
	"github.com/hackclub/hackatime/models"
	"gorm.io/gorm"
)

type gormMigrationFunc func(db *gorm.DB) error

type migrationFunc struct {
	f    func(db *gorm.DB, cfg *config.Config) error
	name string
}

type migrationFuncs []migrationFunc

var (
	preMigrations  migrationFuncs
	postMigrations migrationFuncs
)

func GetMigrationFunc(cfg *config.Config) gormMigrationFunc {
	switch cfg.Db.Dialect {
	default:
		return func(db *gorm.DB) error {
			models := []interface{}{
				&models.User{},
				&models.KeyStringValue{},
				&models.Alias{},
				&models.Heartbeat{},
				&models.Summary{},
				&models.SummaryItem{},
				&models.LanguageMapping{},
				&models.ProjectLabel{},
				&models.Diagnostics{},
				&models.LeaderboardItem{},
				&models.DataDump{},
			}

			for _, model := range models {
				if err := db.AutoMigrate(model); err != nil && !cfg.Db.AutoMigrateFailSilently {
					return err
				}
			}
			return nil
		}
	}
}

func registerPreMigration(f migrationFunc) {
	preMigrations = append(preMigrations, f)
}

func registerPostMigration(f migrationFunc) {
	postMigrations = append(postMigrations, f)
}

func Run(db *gorm.DB, cfg *config.Config) {
	RunPreMigrations(db, cfg)
	RunSchemaMigrations(db, cfg)
	RunPostMigrations(db, cfg)
}

func RunSchemaMigrations(db *gorm.DB, cfg *config.Config) {
	if err := GetMigrationFunc(cfg)(db); err != nil {
		config.Log().Fatal("migration failed", "error", err)
	}
}

func RunPreMigrations(db *gorm.DB, cfg *config.Config) {
	sort.Sort(preMigrations)

	for _, m := range preMigrations {
		slog.Info("potentially running migration", "name", m.name)
		if err := m.f(db, cfg); err != nil {
			config.Log().Fatal("migration failed", "name", m.name, "error", err)
		}
	}
}

func RunPostMigrations(db *gorm.DB, cfg *config.Config) {
	sort.Sort(postMigrations)

	for _, m := range postMigrations {
		slog.Info("potentially running migration", "name", m.name)
		if err := m.f(db, cfg); err != nil {
			config.Log().Fatal("migration failed", "name", m.name, "error", err)
		}
	}
}

func (m migrationFuncs) Len() int {
	return len(m)
}

func (m migrationFuncs) Less(i, j int) bool {
	return strings.Compare(m[i].name, m[j].name) < 0
}

func (m migrationFuncs) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRankingRequestBuckets(t *testing.T) {
	for _, engine := range []struct {
		name string
		env  string
		kind common.DatabaseType
	}{
		{"sqlite", "", common.DatabaseTypeSQLite},
		{"mysql", "TEST_RANKINGS_MYSQL_DSN", common.DatabaseTypeMySQL},
		{"postgres", "TEST_RANKINGS_POSTGRES_DSN", common.DatabaseTypePostgreSQL},
	} {
		t.Run(engine.name, func(t *testing.T) {
			dsn := os.Getenv(engine.env)
			if engine.env != "" && dsn == "" {
				t.Skip("isolated test database is not configured")
			}
			var dialect gorm.Dialector = sqlite.Open(":memory:")
			switch engine.name {
			case "mysql":
				dialect = mysql.Open(dsn)
			case "postgres":
				dialect = postgres.Open(dsn)
			}
			db, err := gorm.Open(dialect, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previous, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetDatabaseTypes(engine.kind, common.LogDatabaseType())
			t.Cleanup(func() {
				DB = previous
				common.SetDatabaseTypes(previousType, common.LogDatabaseType())
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&QuotaData{}))
			require.NoError(t, db.Where("1 = 1").Delete(&QuotaData{}).Error)
			require.NoError(t, db.Create(&[]QuotaData{
				{ModelName: "a", CreatedAt: 7260, Count: 2, TokenUsed: 100},
				{ModelName: "a", CreatedAt: 7320, Count: 3, TokenUsed: 50},
				{ModelName: "b", CreatedAt: 7320, Count: 4},
				{ModelName: "c", CreatedAt: 7320, Count: 6},
				{ModelName: "b", CreatedAt: 10800, Count: 1, TokenUsed: 10},
				{ModelName: "a", CreatedAt: 3600, Count: 100, TokenUsed: 500},
				{ModelName: "", CreatedAt: 7320, Count: 100},
			}).Error)
			rows, err := GetRankingQuotaBuckets(7200, 10800, 3600)
			require.NoError(t, err)
			assert.ElementsMatch(t, []RankingQuotaBucket{
				{ModelName: "a", Bucket: 7200, Tokens: 150, Requests: 5},
				{ModelName: "b", Bucket: 7200, Requests: 4},
				{ModelName: "c", Bucket: 7200, Requests: 6},
				{ModelName: "b", Bucket: 10800, Tokens: 10, Requests: 1},
			}, rows)
			totals, err := GetRankingQuotaTotals(7200, 10800)
			require.NoError(t, err)
			assert.ElementsMatch(t, []RankingQuotaTotal{
				{ModelName: "a", TotalTokens: 150},
				{ModelName: "b", TotalTokens: 10},
				{ModelName: "c", TotalTokens: 0},
			}, totals)
			query := "SELECT version()"
			if engine.name == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
		})
	}
}

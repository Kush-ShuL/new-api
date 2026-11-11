package model

import (
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRootIdentityGuardDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			connection, err := db.DB()
			require.NoError(t, err)
			connection.SetMaxOpenConns(1)
			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				assert.NoError(t, connection.Close())
			})
			require.NoError(t, db.AutoMigrate(&User{}, &UserAccessToken{}, &UserSession{}, &Token{}, &AuthFlow{}, &TwoFA{}, &TwoFABackupCode{}, &PasskeyCredential{}, &UserOAuthBinding{}, &ExternalIdentityClaim{}))
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			useUserCacheMiniRedis(t)

			root := User{Id: 1, Username: "guard-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1, AffCode: "guard-root"}
			ordinary := User{Id: 2, Username: "guard-common", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AuthVersion: 1, AffCode: "guard-common"}
			legacy := "guard-legacy"
			rogue := User{Id: 3, Username: "guard-rogue", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1, AffCode: "guard-rogue", AccessToken: &legacy}
			require.NoError(t, db.Create(&[]User{root, ordinary, rogue}).Error)
			t.Cleanup(func() {
				assert.NoError(t, db.Unscoped().Where("id IN ?", []int{1, 2, 3}).Delete(&User{}).Error)
				assert.NoError(t, db.Unscoped().Where("user_id = ?", 3).Delete(&UserSession{}).Error)
				assert.NoError(t, db.Where("user_id = ?", 3).Delete(&UserAccessToken{}).Error)
			})
			require.NoError(t, db.Create(&UserAccessToken{UserId: 3, TokenHash: "guard-pat", Name: "guard"}).Error)
			session := UserSession{SID: "guard-session", UserID: 3, Version: 1, UserAuthVersion: 1, Status: UserSessionStatusActive, RefreshHash: "guard-hash", ExpiresAt: time.Now().Add(time.Hour).Unix()}
			require.NoError(t, db.Create(&session).Error)
			require.NoError(t, populateUserCache(rogue))
			_, err = GetUserCache(rogue.Id)
			require.ErrorIs(t, err, ErrUnauthorizedRootUser)
			stored, err := GetUserById(rogue.Id, false)
			require.NoError(t, err)
			assert.Equal(t, common.UserStatusDisabled, stored.Status)
			assert.Equal(t, int64(2), stored.AuthVersion)
			assert.Nil(t, stored.AccessToken)
			var count int64
			require.NoError(t, db.Model(&UserAccessToken{}).Where("user_id = ?", 3).Count(&count).Error)
			assert.Zero(t, count)
			require.NoError(t, db.First(&session, "sid = ?", session.SID).Error)
			assert.Equal(t, UserSessionStatusRevoked, session.Status)
			_, err = GetUserCache(rogue.Id)
			assert.ErrorIs(t, err, ErrUnauthorizedRootUser)
			alerts, err := QuarantineUnauthorizedRootUsers()
			require.NoError(t, err)
			assert.Equal(t, []UnauthorizedRootUser{{ID: 3, Username: rogue.Username, Status: common.UserStatusDisabled}}, alerts)
			stored, err = GetUserById(rogue.Id, false)
			require.NoError(t, err)
			assert.Equal(t, int64(2), stored.AuthVersion)
			for _, id := range []int{root.Id, ordinary.Id} {
				user, err := GetUserCache(id)
				require.NoError(t, err)
				assert.Equal(t, common.UserStatusEnabled, user.Status)
			}
			_, err = HardDeleteUnauthorizedRootUserByID(1)
			assert.ErrorIs(t, err, ErrUnauthorizedRootUser)
			_, err = root.HardDelete()
			assert.ErrorIs(t, err, ErrCannotDeleteRootUser)
			_, err = HardDeleteUnauthorizedRootUserByID(2)
			assert.ErrorIs(t, err, ErrUnauthorizedRootUser)
			_, err = HardDeleteUnauthorizedRootUserByID(3)
			require.NoError(t, err)
			require.NoError(t, db.Unscoped().Model(&User{}).Where("id = ?", 3).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

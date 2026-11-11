package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUploadedImagesAndAvatarsDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				if os.Getenv("TEST_MYSQL_DSN") == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(os.Getenv("TEST_MYSQL_DSN"))
			case "postgres":
				if os.Getenv("TEST_POSTGRES_DSN") == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(os.Getenv("TEST_POSTGRES_DSN"))
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			connection, err := db.DB()
			require.NoError(t, err)
			connection.SetMaxOpenConns(1)
			previousDB, previousRedis, previousType := DB, common.RedisEnabled, common.MainDatabaseType()
			DB, common.RedisEnabled = db, false
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				DB, common.RedisEnabled = previousDB, previousRedis
				common.SetMainDatabaseType(previousType)
				initCol()
				assert.NoError(t, connection.Close())
			})
			require.NoError(t, db.AutoMigrate(&User{}, &Option{}, &Model{}, &Vendor{}, &Channel{}, &Ability{}))
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			require.NoError(t, os.MkdirAll(common.UploadedImageDirectory, 0755))
			imagePath := func(url string) string { return filepath.Join(common.UploadedImageDirectory, filepath.Base(url)) }
			writeImage := func(letter string) string {
				url := common.UploadedImageURLPrefix + strings.Repeat(letter, 64) + ".png"
				require.NoError(t, os.WriteFile(imagePath(url), []byte("image fixture"), 0600))
				return url
			}
			shared, replacement := writeImage("a"), writeImage("b")
			user := User{Id: 1, Username: "avatar-user", Password: "password-hash", AffCode: "avatar-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 2500, AuthVersion: 3}
			user.SetSetting(dto.UserSetting{AvatarURL: shared, NotifyType: "email", Language: "zh", RecordIpLog: true})
			user.Setting = strings.TrimSuffix(user.Setting, "}") + `,"custom_profile":{"owner":"custom"},"large_integer":9007199254740993}`
			require.NoError(t, db.Create(&user).Error)
			require.NoError(t, db.Create(&Option{Key: "Logo", Value: shared}).Error)
			metadata := Model{ModelName: "avatar-model", Icon: shared, Status: 1, SyncOfficial: 1}
			vendor := Vendor{Name: "avatar-vendor", Icon: shared, Status: 1}
			require.NoError(t, db.Create(&metadata).Error)
			require.NoError(t, db.Create(&vendor).Error)
			CleanupReplacedUploadedImages(shared)
			assert.FileExists(t, imagePath(shared))
			updated, err := UpdateUserAvatar(user.Id, replacement)
			require.NoError(t, err)
			assert.Equal(t, replacement, updated.GetSetting().AvatarURL)
			assert.Equal(t, "email", updated.GetSetting().NotifyType)
			assert.Equal(t, "zh", updated.GetSetting().Language)
			assert.True(t, updated.GetSetting().RecordIpLog)
			assert.Contains(t, updated.Setting, `"large_integer":9007199254740993`)
			assert.Contains(t, updated.Setting, `"custom_profile":{"owner":"custom"}`)
			assert.True(t, updated.HasPassword)
			assert.Equal(t, 2500, updated.Quota)
			assert.Equal(t, int64(3), updated.AuthVersion)
			assert.FileExists(t, imagePath(shared))
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "Logo").Update("value", "").Error)
			CleanupReplacedUploadedImages(shared)
			assert.FileExists(t, imagePath(shared))
			metadata.Icon = ""
			require.NoError(t, metadata.Update())
			assert.FileExists(t, imagePath(shared))
			vendor.Icon = ""
			require.NoError(t, vendor.Update())
			assert.NoFileExists(t, imagePath(shared))
			_, err = UpdateUserAvatar(user.Id, common.UploadedImageURLPrefix+strings.Repeat("f", 64)+".png")
			require.Error(t, err)

			abandoned, recent := writeImage("c"), writeImage("d")
			old := time.Now().Add(-48 * time.Hour)
			require.NoError(t, os.Chtimes(imagePath(abandoned), old, old))
			require.NoError(t, os.Chtimes(imagePath(replacement), old, old))
			partial := filepath.Join(common.UploadedImageDirectory, ".upload-abandoned")
			require.NoError(t, os.WriteFile(partial, []byte("partial"), 0600))
			require.NoError(t, os.Chtimes(partial, old, old))
			removed, err := SweepUnusedUploadedImages()
			require.NoError(t, err)
			assert.Equal(t, 2, removed)
			assert.NoFileExists(t, imagePath(abandoned))
			assert.FileExists(t, imagePath(recent))
			assert.FileExists(t, imagePath(replacement))
			_, err = UpdateUserAvatar(user.Id, "")
			require.NoError(t, err)
			assert.NoFileExists(t, imagePath(replacement))
			stored, err := GetUserById(user.Id, false)
			require.NoError(t, err)
			assert.Empty(t, stored.GetSetting().AvatarURL)
			assert.Equal(t, common.UserStatusEnabled, stored.Status)
			// A failed reference scan must never remove a still-unknown image.
			require.NoError(t, db.Migrator().DropTable(&Model{}))
			CleanupReplacedUploadedImages(recent)
			assert.FileExists(t, imagePath(recent))
		})
	}
}

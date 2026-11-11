package model

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var uploadedImageReference = regexp.MustCompile(`[a-f0-9]{64}\.(png|jpg|gif|webp)`)

func UploadedImageReferences() (map[string]struct{}, error) {
	references := make(map[string]struct{})
	var values []*string
	for _, field := range []struct {
		record any
		column string
	}{
		{&Option{}, "value"}, {&User{}, "setting"},
		{&Model{}, "icon"}, {&Model{}, "description"},
		{&Vendor{}, "icon"}, {&Vendor{}, "description"},
	} {
		values = nil
		if err := DB.Model(field.record).Pluck(field.column, &values).Error; err != nil {
			return nil, err
		}
		for _, value := range values {
			if value == nil {
				continue
			}
			for _, name := range uploadedImageReference.FindAllString(*value, -1) {
				references[name] = struct{}{}
			}
		}
	}
	return references, nil
}

// A content-addressed image may be shared by settings, model cards and users.
// Delete a replaced file only after every persisted reference has gone away.
func CleanupReplacedUploadedImages(previous ...string) {
	names := make(map[string]struct{})
	for _, value := range previous {
		for _, name := range uploadedImageReference.FindAllString(value, -1) {
			names[name] = struct{}{}
		}
	}
	if len(names) == 0 {
		return
	}
	common.UploadedImageMutex.Lock()
	defer common.UploadedImageMutex.Unlock()
	references, err := UploadedImageReferences()
	if err != nil {
		common.SysError(common.LogText("Failed to check uploaded image references: %v", err))
		return
	}
	for name := range names {
		if _, used := references[name]; used {
			continue
		}
		if err := os.Remove(filepath.Join(common.UploadedImageDirectory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			common.SysError(common.LogText("Failed to remove unused uploaded image: %v", err))
		}
	}
}

// Unclaimed uploads get a day to be saved. Referenced images never expire.
func SweepUnusedUploadedImages() (int, error) {
	common.UploadedImageMutex.Lock()
	defer common.UploadedImageMutex.Unlock()
	entries, err := os.ReadDir(common.UploadedImageDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	references, err := UploadedImageReferences()
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!common.UploadedImageName.MatchString(name) && !strings.HasPrefix(name, ".upload-")) {
			continue
		}
		if _, used := references[name]; used {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if time.Since(info.ModTime()) < 24*time.Hour {
			continue
		}
		if err := os.Remove(filepath.Join(common.UploadedImageDirectory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func UpdateUserAvatar(userID int, avatarURL string) (*User, error) {
	var user User
	var previous string
	common.UploadedImageMutex.Lock()
	if avatarURL != "" {
		name, local := strings.CutPrefix(avatarURL, common.UploadedImageURLPrefix)
		if !local || !common.UploadedImageName.MatchString(name) {
			common.UploadedImageMutex.Unlock()
			return nil, common.NewMessage("Invalid avatar image")
		}
		if _, err := os.Stat(filepath.Join(common.UploadedImageDirectory, name)); err != nil {
			common.UploadedImageMutex.Unlock()
			return nil, common.NewMessage("Invalid avatar image")
		}
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		previous = user.GetSetting().AvatarURL
		settings := make(map[string]json.RawMessage)
		if user.Setting != "" {
			if err := common.UnmarshalJsonStr(user.Setting, &settings); err != nil {
				return err
			}
			if settings == nil {
				settings = make(map[string]json.RawMessage)
			}
		}
		if avatarURL == "" {
			delete(settings, "avatar_url")
		} else {
			encodedAvatar, err := common.Marshal(avatarURL)
			if err != nil {
				return err
			}
			settings["avatar_url"] = encodedAvatar
		}
		encoded, err := common.Marshal(settings)
		if err != nil {
			return err
		}
		user.Setting = string(encoded)
		return tx.Model(&user).Update("setting", user.Setting).Error
	})
	common.UploadedImageMutex.Unlock()
	if err != nil {
		return nil, err
	}
	if err := PublishUserAuthCache(userID); err != nil {
		return nil, err
	}
	user.HasPassword = user.Password != ""
	CleanupReplacedUploadedImages(previous)
	return &user, nil
}

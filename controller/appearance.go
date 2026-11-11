/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

const maxBackgroundImageBytes = 100 << 20
const backgroundImageDirectory = common.UploadedImageDirectory
const maxAvatarImageBytes = 5 << 20

var backgroundImageName = common.UploadedImageName
var backgroundUploadID = regexp.MustCompile(`^[a-f0-9-]{36}$`)

// Uploads live alongside the mounted data directory; options contain only a URL.
func UploadAppearanceBackground(c *gin.Context) {
	uploadProfileImage(c, maxBackgroundImageBytes)
}

func UploadUserAvatar(c *gin.Context) {
	uploadProfileImage(c, maxAvatarImageBytes)
}

func uploadProfileImage(c *gin.Context, maxBytes int64) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 9<<20)
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	uploadID := c.Query("upload_id")
	offset, offsetErr := strconv.ParseInt(c.Query("offset"), 10, 64)
	total, totalErr := strconv.ParseInt(c.Query("total"), 10, 64)
	header, err := c.FormFile("image")
	if err != nil || offsetErr != nil || totalErr != nil || !backgroundUploadID.MatchString(uploadID) ||
		offset < 0 || total <= 0 || total > maxBytes || header.Size <= 0 ||
		header.Size > 8<<20 || offset+header.Size > total {
		common.ApiErrorT(c, "Choose a PNG, JPEG, WebP, or GIF image up to {{size}} MB.", map[string]any{"size": maxBytes >> 20})
		return
	}
	source, err := header.Open()
	if err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	defer source.Close()
	common.UploadedImageMutex.Lock()
	defer common.UploadedImageMutex.Unlock()
	if err = os.MkdirAll(backgroundImageDirectory, 0755); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	path := filepath.Join(backgroundImageDirectory, fmt.Sprintf(".upload-%d-%s", c.GetInt("id"), uploadID))
	flags := os.O_RDWR
	if offset == 0 {
		flags |= os.O_CREATE | os.O_EXCL
		entries, _ := os.ReadDir(backgroundImageDirectory)
		for _, entry := range entries {
			info, statErr := entry.Info()
			if statErr == nil && strings.HasPrefix(entry.Name(), ".upload-") && time.Since(info.ModTime()) > 24*time.Hour {
				_ = os.Remove(filepath.Join(backgroundImageDirectory, entry.Name()))
			}
		}
	}
	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() != offset {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	if _, err = file.Seek(offset, io.SeekStart); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	if _, err = io.Copy(file, source); err != nil {
		_ = file.Truncate(offset)
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	if offset+header.Size < total {
		common.ApiSuccess(c, gin.H{"url": ""})
		return
	}
	defer os.Remove(path)
	if maxBytes == maxAvatarImageBytes {
		if err := validateAvatarImage(file); err != nil {
			common.ApiErrorT(c, "Invalid avatar image")
			return
		}
	}
	_, _ = file.Seek(0, io.SeekStart)
	var prefix [512]byte
	n, _ := io.ReadFull(file, prefix[:])
	extensions := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp", "image/gif": "gif"}
	extension, ok := extensions[http.DetectContentType(prefix[:n])]
	if !ok {
		common.ApiErrorT(c, "Background image must be PNG, JPEG, WebP, or GIF and no larger than 100 MB.")
		return
	}
	_, _ = file.Seek(0, io.SeekStart)
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	if err = file.Close(); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	name := fmt.Sprintf("%x.%s", hash.Sum(nil), extension)
	if _, err = os.Stat(filepath.Join(backgroundImageDirectory, name)); err == nil {
		common.ApiSuccess(c, gin.H{"url": "/api/appearance/background/" + name})
		return
	}
	if err = os.Rename(path, filepath.Join(backgroundImageDirectory, name)); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	common.ApiSuccess(c, gin.H{"url": "/api/appearance/background/" + name})
}

func validateAvatarImage(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxAvatarImageBytes {
		return fmt.Errorf("avatar image is too large")
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return err
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
		return fmt.Errorf("invalid avatar dimensions")
	}
	return nil
}

func UpdateUserAvatar(c *gin.Context) {
	var request struct {
		URL string `json:"url"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	// Avatars use our authenticated upload path; external image fetching is
	// unnecessary and replacing a URL cannot overwrite another user's files.
	if request.URL != "" {
		name, local := strings.CutPrefix(request.URL, common.UploadedImageURLPrefix)
		if !local || !backgroundImageName.MatchString(name) {
			common.ApiErrorT(c, "Invalid avatar image")
			return
		}
		file, err := os.Open(filepath.Join(backgroundImageDirectory, name))
		if err != nil {
			common.ApiErrorT(c, "Invalid avatar image")
			return
		}
		err = validateAvatarImage(file)
		_ = file.Close()
		if err != nil {
			common.ApiErrorT(c, "Invalid avatar image")
			return
		}
	}
	user, err := model.UpdateUserAvatar(c.GetInt("id"), request.URL)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildSelfUserData(user))
}

type userWithAvatar struct {
	*model.User
	AvatarURL string `json:"avatar_url,omitempty"`
}

func usersWithAvatars(users []*model.User) []userWithAvatar {
	result := make([]userWithAvatar, 0, len(users))
	for _, user := range users {
		result = append(result, userWithAvatar{User: user, AvatarURL: user.GetSetting().AvatarURL})
	}
	return result
}

type logWithAvatar struct {
	*model.Log
	AvatarURL string `json:"avatar_url,omitempty"`
}

func logsWithAvatars(logs []*model.Log) []logWithAvatar {
	ids := make(map[int]struct{})
	for _, entry := range logs {
		ids[entry.UserId] = struct{}{}
	}
	userIDs := make([]int, 0, len(ids))
	for id := range ids {
		if id > 0 {
			userIDs = append(userIDs, id)
		}
	}
	var users []*model.User
	if len(userIDs) > 0 {
		if err := model.DB.Select("id", "setting").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			common.SysError(common.LogText("Failed to load user avatars: %v", err))
		}
	}
	avatars := make(map[int]string, len(users))
	for _, user := range users {
		avatars[user.Id] = user.GetSetting().AvatarURL
	}
	result := make([]logWithAvatar, 0, len(logs))
	for _, entry := range logs {
		result = append(result, logWithAvatar{Log: entry, AvatarURL: avatars[entry.UserId]})
	}
	return result
}

func GetAppearanceBackground(c *gin.Context) {
	name := c.Param("name")
	if !backgroundImageName.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(filepath.Join(backgroundImageDirectory, name))
}

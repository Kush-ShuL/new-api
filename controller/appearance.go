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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const maxBackgroundImageBytes = 100 << 20
const backgroundImageDirectory = "uploads/backgrounds"

var backgroundImageName = regexp.MustCompile(`^[a-f0-9]{64}\.(png|jpg|gif|webp)$`)
var backgroundUploadID = regexp.MustCompile(`^[a-f0-9-]{36}$`)
var backgroundUploadMutex sync.Mutex

// Uploads live alongside the mounted data directory; options contain only a URL.
func UploadAppearanceBackground(c *gin.Context) {
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
		offset < 0 || total <= 0 || total > maxBackgroundImageBytes || header.Size <= 0 ||
		header.Size > 8<<20 || offset+header.Size > total {
		common.ApiErrorT(c, "Background image must be PNG, JPEG, WebP, or GIF and no larger than 100 MB.")
		return
	}
	source, err := header.Open()
	if err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	defer source.Close()
	backgroundUploadMutex.Lock()
	defer backgroundUploadMutex.Unlock()
	if err = os.MkdirAll(backgroundImageDirectory, 0755); err != nil {
		common.ApiErrorT(c, "Failed to upload image")
		return
	}
	path := filepath.Join(backgroundImageDirectory, ".upload-"+uploadID)
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

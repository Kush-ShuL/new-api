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
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postBackgroundChunk(t *testing.T, router *gin.Engine, id string, offset, total int, content []byte) map[string]any {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "background.png")
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/upload?upload_id=%s&offset=%d&total=%d", id, offset, total), &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result map[string]any
	require.NoError(t, common.DecodeJson(response.Body, &result))
	return result
}

func TestBackgroundUploadAccepts100MiBInChunksAndServesAnImmutableURL(t *testing.T) {
	t.Chdir(t.TempDir())
	router := gin.New()
	router.POST("/upload", UploadAppearanceBackground)
	router.GET("/api/appearance/background/:name", GetAppearanceBackground)
	chunk := make([]byte, 8<<20)
	copy(chunk, []byte("\x89PNG\r\n\x1a\n"))
	var url string
	for offset := 0; offset < maxBackgroundImageBytes; offset += len(chunk) {
		length := min(len(chunk), maxBackgroundImageBytes-offset)
		result := postBackgroundChunk(t, router, "12345678-1234-1234-1234-123456789abc", offset, maxBackgroundImageBytes, chunk[:length])
		require.Equal(t, true, result["success"])
		url = result["data"].(map[string]any)["url"].(string)
		if offset+length < maxBackgroundImageBytes {
			assert.Empty(t, url)
		}
	}
	require.NotEmpty(t, url)
	info, err := os.Stat(filepath.Join(backgroundImageDirectory, filepath.Base(url)))
	require.NoError(t, err)
	assert.EqualValues(t, maxBackgroundImageBytes, info.Size())
	request := httptest.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("Range", "bytes=0-7")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusPartialContent, response.Code)
	assert.Equal(t, "\x89PNG\r\n\x1a\n", response.Body.String())
	assert.Contains(t, response.Header().Get("Cache-Control"), "immutable")
}

func TestBackgroundUploadRejectsInvalidTypeSizeAndOffsets(t *testing.T) {
	t.Chdir(t.TempDir())
	router := gin.New()
	router.POST("/upload", UploadAppearanceBackground)
	for _, test := range []struct {
		name, id      string
		offset, total int
		body          string
	}{
		{"too large", "12345678-1234-1234-1234-123456789abc", 0, maxBackgroundImageBytes + 1, "x"},
		{"invalid type", "22345678-1234-1234-1234-123456789abc", 0, 6, "<svg/>"},
		{"out of order", "32345678-1234-1234-1234-123456789abc", 8, 9, "x"},
		{"invalid path", strings.Repeat("../", 12), 0, 1, "x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := postBackgroundChunk(t, router, test.id, test.offset, test.total, []byte(test.body))
			assert.Equal(t, false, result["success"])
		})
	}
}

func TestAvatarUploadValidatesImagesLimitsAndChunkOwnership(t *testing.T) {
	t.Chdir(t.TempDir())
	userID := 1
	router := gin.New()
	router.POST("/upload", func(c *gin.Context) { c.Set("id", userID); UploadUserAvatar(c) })
	var content bytes.Buffer
	require.NoError(t, png.Encode(&content, image.NewNRGBA(image.Rect(0, 0, 4, 4))))
	data := content.Bytes()
	id := "12345678-1234-1234-1234-123456789abc"
	first := postBackgroundChunk(t, router, id, 0, len(data), data[:8])
	require.Equal(t, true, first["success"])
	userID = 2
	assert.Equal(t, false, postBackgroundChunk(t, router, id, 8, len(data), data[8:])["success"])
	userID = 1
	complete := postBackgroundChunk(t, router, id, 8, len(data), data[8:])
	require.Equal(t, true, complete["success"])
	assert.NotEmpty(t, complete["data"].(map[string]any)["url"])
	for _, test := range []struct {
		id    string
		total int
		data  []byte
	}{
		{"22345678-1234-1234-1234-123456789abc", 6, []byte("<svg/>")},
		{"32345678-1234-1234-1234-123456789abc", maxAvatarImageBytes + 1, []byte("x")},
		{"42345678-1234-1234-1234-123456789abc", 8, []byte("\x89PNG\r\n\x1a\n")},
	} {
		assert.Equal(t, false, postBackgroundChunk(t, router, test.id, 0, test.total, test.data)["success"])
	}
}

func TestAvatarUpdateUsesAuthenticatedUserAndCleansReplacedImage(t *testing.T) {
	root, _ := setupSecurityEnrollmentTest(t)
	t.Chdir(t.TempDir())
	require.NoError(t, model.DB.AutoMigrate(&model.Model{}, &model.Vendor{}))
	other := &model.User{Username: "avatar-other", AffCode: "avatar-other", Password: root.Password, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, model.DB.Create(other).Error)
	bundle, err := service.CreateLoginSession(other.Id, "password", "127.0.0.1", "avatar-test")
	require.NoError(t, err)
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 4, 4))))
	name := fmt.Sprintf("%x.png", sha256.Sum256(data.Bytes()))
	require.NoError(t, os.MkdirAll(backgroundImageDirectory, 0755))
	path := filepath.Join(backgroundImageDirectory, name)
	require.NoError(t, os.WriteFile(path, data.Bytes(), 0600))
	url := common.UploadedImageURLPrefix + name
	router := gin.New()
	router.PUT("/api/user/self/avatar", middleware.UserAuth(), UpdateUserAvatar)
	request := httptest.NewRequest(http.MethodPut, "/api/user/self/avatar", strings.NewReader(`{"url":"`+url+`","user_id":1,"role":100,"quota":999999}`))
	request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"success":true`)
	stored, err := model.GetUserById(other.Id, false)
	require.NoError(t, err)
	assert.Equal(t, url, stored.GetSetting().AvatarURL)
	assert.Equal(t, common.RoleCommonUser, stored.Role)
	assert.Zero(t, stored.Quota)
	unchanged, err := model.GetUserById(root.Id, false)
	require.NoError(t, err)
	assert.Empty(t, unchanged.GetSetting().AvatarURL)
	for _, invalid := range []string{"javascript:alert(1)", "../../secret.png", common.UploadedImageURLPrefix + strings.Repeat("f", 64) + ".png"} {
		request = httptest.NewRequest(http.MethodPut, "/api/user/self/avatar", strings.NewReader(`{"url":"`+invalid+`"}`))
		request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Contains(t, response.Body.String(), `"success":false`)
	}
	request = httptest.NewRequest(http.MethodPut, "/api/user/self/avatar", strings.NewReader(`{"url":""}`))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	request = httptest.NewRequest(http.MethodPut, "/api/user/self/avatar", strings.NewReader(`{"url":""}`))
	request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"success":true`)
	assert.NoFileExists(t, path)
}

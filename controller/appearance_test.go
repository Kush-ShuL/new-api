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
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

package service

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	responsesContextTTL      = 24 * time.Hour
	responsesReasoningPrefix = "nv1."
)

var responsesCacheSweep struct {
	sync.Mutex
	last time.Time
}

type ResponsesStoredContext struct {
	Input     []common.RawMessage `json:"input"`
	Response  common.RawMessage   `json:"response"`
	ExpiresAt int64               `json:"expires_at"`
}

// A store=false WebSocket turn retains only the last response on that client
// connection. Persistent contexts use Redis when enabled, otherwise disk.
type ResponsesSessionContext struct {
	ID      string
	Context *ResponsesStoredContext
}

func responsesCachePath(userID int, responseID string) string {
	directory := os.Getenv("RESPONSES_CACHE_DIR")
	if directory == "" {
		directory = "responses-cache"
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", userID, responseID)))
	return filepath.Join(directory, fmt.Sprintf("%x.json", hash))
}

func LoadResponsesContext(c *gin.Context, id string) (*ResponsesStoredContext, error) {
	if session, ok := c.Get("responses_session_context"); ok {
		cached := session.(*ResponsesSessionContext)
		if cached.ID == id && cached.Context != nil && cached.Context.ExpiresAt > time.Now().Unix() {
			return cached.Context, nil
		}
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	path := responsesCachePath(userID, id)
	var data []byte
	var err error
	if common.RedisEnabled {
		data, err = common.RDB.Get(c.Request.Context(), "responses:context:"+filepath.Base(path)).Bytes()
	} else {
		data, err = os.ReadFile(path)
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, redis.Nil) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	var stored ResponsesStoredContext
	if err := common.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	if stored.ExpiresAt <= time.Now().Unix() {
		return nil, os.ErrNotExist
	}
	return &stored, nil
}

func SaveResponsesContext(c *gin.Context, id string, request *dto.OpenAIResponsesRequest, response []byte) error {
	input, err := responsesInputItems(request.Input)
	if err != nil {
		return err
	}
	stored := &ResponsesStoredContext{Input: input, Response: response, ExpiresAt: time.Now().Add(responsesContextTTL).Unix()}
	data, err := common.Marshal(stored)
	if err != nil {
		return err
	}
	if len(data) > responsesContextMaxBytes() {
		return errors.New("Responses context exceeds the cache size limit")
	}
	if value, ok := c.Get("responses_session_context"); ok {
		session := value.(*ResponsesSessionContext)
		session.ID, session.Context = id, stored
	}
	if strings.TrimSpace(string(request.Store)) == "false" {
		return nil
	}
	// Generation may have finished while the peer disconnected. Persistence
	// still gets a short, bounded opportunity to save that completed response.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	path := responsesCachePath(common.GetContextKeyInt(c, constant.ContextKeyUserId), id)
	if common.RedisEnabled {
		return common.RDB.Set(ctx, "responses:context:"+filepath.Base(path), data, responsesContextTTL).Err()
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".response-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	responsesCacheSweep.Lock()
	if time.Since(responsesCacheSweep.last) < time.Hour {
		responsesCacheSweep.Unlock()
		return nil
	}
	responsesCacheSweep.last = time.Now()
	responsesCacheSweep.Unlock()
	go func() {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > responsesContextTTL {
				_ = os.Remove(filepath.Join(directory, entry.Name()))
			}
		}
	}()
	return nil
}

func DeleteResponsesContext(c *gin.Context, id string) error {
	if _, err := LoadResponsesContext(c, id); err != nil {
		return err
	}
	path := responsesCachePath(common.GetContextKeyInt(c, constant.ContextKeyUserId), id)
	if common.RedisEnabled {
		return common.RDB.Del(c.Request.Context(), "responses:context:"+filepath.Base(path)).Err()
	}
	return os.Remove(path)
}

func responsesContextMaxBytes() int {
	maxMB := constant.MaxRequestBodyMB
	if maxMB <= 0 {
		maxMB = 128
	}
	return maxMB << 20
}

func responsesInputItems(raw common.RawMessage) ([]common.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if common.Unmarshal(raw, &text) == nil {
		item, err := common.Marshal(map[string]any{"role": "user", "content": text})
		return []common.RawMessage{item}, err
	}
	var items []common.RawMessage
	err := common.Unmarshal(raw, &items)
	return items, err
}

// PrepareStatefulResponsesRequest resolves local response chains before token
// estimation. An ID must belong to the authenticated user in the local cache.
func PrepareStatefulResponsesRequest(c *gin.Context, request *dto.OpenAIResponsesRequest) error {
	input, err := responsesInputItems(request.Input)
	if err != nil {
		return err
	}
	if request.PreviousResponseID != "" {
		stored, err := LoadResponsesContext(c, request.PreviousResponseID)
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("previous_response_id was not found or has expired")
		}
		if err != nil {
			return err
		}
		if stored != nil {
			var response struct {
				Output []common.RawMessage `json:"output"`
			}
			if err := common.Unmarshal(stored.Response, &response); err != nil {
				return err
			}
			combined := make([]common.RawMessage, 0, len(stored.Input)+len(response.Output)+len(input))
			combined = append(combined, stored.Input...)
			combined = append(combined, response.Output...)
			input = append(combined, input...)
			request.PreviousResponseID = ""
			c.Set("responses_input_rewritten", true)
		}
	}
	for i, raw := range input {
		var item map[string]any
		if common.Unmarshal(raw, &item) != nil {
			return errors.New("invalid Responses input item")
		}
		encrypted, _ := item["encrypted_content"].(string)
		if !strings.HasPrefix(encrypted, responsesReasoningPrefix) {
			continue
		}
		text, err := OpenResponsesReasoning(common.GetContextKeyInt(c, constant.ContextKeyUserId), encrypted)
		if err != nil {
			return err
		}
		delete(item, "encrypted_content")
		c.Set("responses_input_rewritten", true)
		item["summary"] = []map[string]string{{"type": "summary_text", "text": text}}
		input[i], err = common.Marshal(item)
		if err != nil {
			return err
		}
	}
	if len(input) > 0 {
		request.Input, err = common.Marshal(input)
		if len(request.Input) > responsesContextMaxBytes() {
			return errors.New("Responses context exceeds the request size limit")
		}
	}
	return err
}

func responsesReasoningCipher() (cipher.AEAD, error) {
	key, err := model.GetResponsesReasoningKey()
	if err != nil {
		return nil, err
	}
	return chacha20poly1305.New(key)
}

func SealResponsesReasoning(userID int, text string) (string, error) {
	aead, err := responsesReasoningCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(text), []byte(strconv.Itoa(userID)))
	return responsesReasoningPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func OpenResponsesReasoning(userID int, encrypted string) (string, error) {
	encrypted, marked := strings.CutPrefix(encrypted, responsesReasoningPrefix)
	if !marked {
		return "", errors.New("invalid encrypted reasoning")
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(encrypted, "="))
	if err != nil || len(data) < chacha20poly1305.NonceSize+chacha20poly1305.Overhead {
		return "", errors.New("invalid encrypted reasoning")
	}
	aead, err := responsesReasoningCipher()
	if err != nil {
		return "", err
	}
	text, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte(strconv.Itoa(userID)))
	if err != nil {
		return "", errors.New("invalid encrypted reasoning")
	}
	return string(text), nil
}

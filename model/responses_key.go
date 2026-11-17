package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetResponsesReasoningKey atomically initializes one shared key across gateway
// instances. It uses the existing options table, without a schema migration.
func GetResponsesReasoningKey() ([]byte, error) {
	const name = "ResponsesReasoningEncryptionKey"
	var option Option
	err := DB.Where(&Option{Key: name}).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		option = Option{Key: name, Value: base64.StdEncoding.EncodeToString(key)}
		if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&option).Error; err != nil {
			return nil, err
		}
		err = DB.Where(&Option{Key: name}).First(&option).Error
	}
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(option.Value)
	if err != nil || len(key) != 32 {
		return nil, errors.New("invalid Responses reasoning encryption key")
	}
	return key, nil
}

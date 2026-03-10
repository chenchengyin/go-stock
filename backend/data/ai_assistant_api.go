package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

const maxSavedMessages = 200

// GetAiAssistantSession 获取最近一次会话的消息列表
func GetAiAssistantSession() ([]models.AiAssistantMessage, error) {
	var row models.AiAssistantSession
	err := db.Dao.Model(&models.AiAssistantSession{}).Order("updated_at DESC").First(&row).Error
	if err != nil {
		// 无记录时返回空切片
		return []models.AiAssistantMessage{}, nil
	}
	if row.Messages == "" {
		return []models.AiAssistantMessage{}, nil
	}
	var list []models.AiAssistantMessage
	if err := json.Unmarshal([]byte(row.Messages), &list); err != nil {
		return []models.AiAssistantMessage{}, nil
	}
	return list, nil
}

// SaveAiAssistantSession 保存会话消息到数据库（只保留最近一条会话，最多 maxSavedMessages 条消息）
func SaveAiAssistantSession(messages []models.AiAssistantMessage) error {
	if len(messages) == 0 {
		return nil
	}
	toSave := messages
	if len(toSave) > maxSavedMessages {
		toSave = toSave[len(toSave)-maxSavedMessages:]
	}
	raw, err := json.Marshal(toSave)
	if err != nil {
		return err
	}
	payload := string(raw)

	var row models.AiAssistantSession
	err = db.Dao.Model(&models.AiAssistantSession{}).Order("updated_at DESC").First(&row).Error
	if err != nil {
		// 无记录则新建
		return db.Dao.Create(&models.AiAssistantSession{Messages: payload}).Error
	}
	return db.Dao.Model(&row).Update("messages", payload).Error
}

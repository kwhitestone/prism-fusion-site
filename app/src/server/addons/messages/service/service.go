package service

import (
	"sync"
	"time"
)

// Message 用户消息结构
type Message struct {
	ID        int       `json:"id" doc:"消息ID"`
	Content   string    `json:"content" doc:"消息内容"`
	Author    string    `json:"author" doc:"作者"`
	CreatedAt time.Time `json:"created_at" doc:"创建时间"`
}

// MessageStore 内存消息存储
type messageStore struct {
	mu       sync.RWMutex
	messages []Message
	nextID   int
}

// 全局消息存储实例
var store = &messageStore{
	messages: make([]Message, 0),
	nextID:   1,
}

type MessagesService struct{}

// AddMessage 添加消息到内存
func (s *MessagesService) AddMessage(content, author string) *Message {
	store.mu.Lock()
	defer store.mu.Unlock()

	msg := Message{
		ID:        store.nextID,
		Content:   content,
		Author:    author,
		CreatedAt: time.Now(),
	}
	store.messages = append(store.messages, msg)
	store.nextID++

	return &msg
}

// GetMessages 获取所有消息
func (s *MessagesService) GetMessages() []Message {
	store.mu.RLock()
	defer store.mu.RUnlock()

	result := make([]Message, len(store.messages))
	copy(result, store.messages)
	return result
}

// DeleteMessage 删除指定消息
func (s *MessagesService) DeleteMessage(id int) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	for i, msg := range store.messages {
		if msg.ID == id {
			store.messages = append(store.messages[:i], store.messages[i+1:]...)
			return true
		}
	}
	return false
}

// ClearMessages 清空所有消息
func (s *MessagesService) ClearMessages() {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.messages = make([]Message, 0)
}

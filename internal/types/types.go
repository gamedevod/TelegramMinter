package types

import "time"

// Sticker представляет стикер
type Sticker struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StickerPack представляет набор стикеров
type StickerPack struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Stickers    []Sticker `json:"stickers"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// User представляет пользователя
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// APIResponse общая структура ответа API
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Error   string      `json:"error,omitempty"`
}

// BuyResponse ответ на запрос покупки стикеров
type BuyResponse struct {
	OK        bool        `json:"ok"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// BuyRequest параметры для покупки стикеров
type BuyRequest struct {
	Collection int    `json:"collection"`
	Character  int    `json:"character"`
	Currency   string `json:"currency"`
	Count      int    `json:"count"`
}

// Statistics статистика покупок
type Statistics struct {
	TotalRequests    int           `json:"total_requests"`
	SuccessRequests  int           `json:"success_requests"`
	FailedRequests   int           `json:"failed_requests"`
	InvalidTokens    int           `json:"invalid_tokens"`
	SentTransactions int           `json:"sent_transactions"`
	StartTime        time.Time     `json:"start_time"`
	Duration         time.Duration `json:"duration"`
	RequestsPerSec   float64       `json:"requests_per_sec"`
}

// AppState состояние приложения
type AppState struct {
	CurrentUser  *User         `json:"current_user"`
	StickerPacks []StickerPack `json:"sticker_packs"`
	CurrentPack  *StickerPack  `json:"current_pack"`
	IsLoggedIn   bool          `json:"is_logged_in"`
	LastUpdated  time.Time     `json:"last_updated"`
	IsRunning    bool          `json:"is_running"`
	Statistics   *Statistics   `json:"statistics"`
}

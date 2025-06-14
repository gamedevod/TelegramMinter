package monitor

// CollectionsResponse ответ API со списком коллекций
type CollectionsResponse struct {
	OK   bool         `json:"ok"`
	Data []Collection `json:"data"`
}

// CollectionDetailsResponse ответ API с деталями коллекции
type CollectionDetailsResponse struct {
	OK   bool              `json:"ok"`
	Data CollectionDetails `json:"data"`
}

// Collection базовая информация о коллекции
type Collection struct {
	ID          int      `json:"id"`
	Creator     Creator  `json:"creator"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Media       []Media  `json:"media"`
	Status      string   `json:"status"`
	Badges      []string `json:"badges"`
}

// CollectionDetails детальная информация о коллекции
type CollectionDetails struct {
	Collection Collection  `json:"collection"`
	Stickers   []Sticker   `json:"stickers"`
	Characters []Character `json:"characters"`
}

// Creator информация о создателе
type Creator struct {
	Name          string       `json:"name"`
	Status        string       `json:"status"`
	SocialLinks   []SocialLink `json:"social_links"`
	RoyaltyWallet string       `json:"royalty_wallet"`
}

// SocialLink социальная ссылка
type SocialLink struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

// Media медиа файл
type Media struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

// Sticker информация о стикере
type Sticker struct {
	ID         int                    `json:"id"`
	Emojis     []string               `json:"emojis"`
	Media      []Media                `json:"media"`
	Format     string                 `json:"format"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Character информация о персонаже
type Character struct {
	ID           int                    `json:"id"`
	CollectionID int                    `json:"collection_id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Stickers     []int                  `json:"stickers"`
	Price        int                    `json:"price"`  // В нанотонах
	Left         int                    `json:"left"`   // Осталось
	Supply       int                    `json:"supply"` // Общее количество
	Type         string                 `json:"type"`
	Attributes   map[string]interface{} `json:"attributes"`
}

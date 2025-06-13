# 🔐 Telegram Web App Authorization

Реализация авторизации через Telegram Web App используя существующий `HTTPClient` из проекта.

## 📋 Описание процесса

### 1. **Получение Auth Data** (`get_auth_data`)
```go
// Аналог Python функции get_auth_data
authResponse, err := webAppService.GetAuthData(ctx, botUsername, webAppURL)
```

**Что происходит:**
- Подключение к Telegram клиенту
- Вызов `MessagesRequestWebView` с параметрами:
  - `peer`: бот
  - `bot`: бот как пользователь  
  - `platform`: "android"
  - `from_bot_menu`: false
  - `url`: URL Web App
- Извлечение `tgWebAppData` из ответа
- Двойное декодирование URL (unquote)
- Создание `AuthData` с временем истечения 45 минут

### 2. **Авторизация через API** (`auth`)
```go
// Аналог Python функции auth используя существующий HTTPClient
httpClient := client.New()
tokenResponse, err := httpClient.AuthenticateWithTelegramData(apiURL, authData)
```

**Что происходит:**
- Используется существующий `HTTPClient` с `tls-client`
- POST запрос на `/auth` endpoint через `client.Post()`
- Отправка auth data в теле запроса
- Получение Bearer токена из ответа
- Сохранение токена для последующих запросов

## 🔧 Архитектура

### HTTP Client Integration
```go
// Используем существующий HTTPClient из проекта
type WebAppService struct {
    api         *tg.Client
    botUsername string
    webAppURL   string
    httpClient  *client.HTTPClient // Существующий HTTP клиент
}
```

### Существующие методы
- `client.New()` - создание HTTP клиента с Chrome профилем
- `client.Post()` - POST запросы через `bogdanfinn/tls-client`
- `client.AuthenticateWithTelegramData()` - новый метод авторизации

## 🚀 Как работает

### Структура AuthData (в client пакете)
```go
type AuthData struct {
    Data string    // Декодированные tgWebAppData
    Exp  time.Time // Время истечения (45 минут)
}
```

### TLS Client Features
- **Chrome 120 профиль** для обхода защит
- **Random TLS Extension Order** для анонимности  
- **Cookie Jar** для сессий
- **30 секунд таймаут**
- **No redirect follow** для контроля

### API запрос через существующий клиент
```go
// Использует HTTPClient.Post() вместо net/http
resp, err := c.Post(fmt.Sprintf("%s/auth", apiURL), formData, headers)
```

## 📊 Интеграция с существующим кодом

| Компонент | Было | Стало |
|-----------|------|-------|
| HTTP клиент | `net/http.Client` | `client.HTTPClient` |
| POST запрос | `http.Post()` | `client.Post()` |
| Структуры | `telegram/auth_data.go` | `client/auth.go` |
| TLS | Стандартный | `bogdanfinn/tls-client` |

## 🔍 Используемые библиотеки

- **gotd/td** - Telegram MTProto API
- **bogdanfinn/tls-client** - HTTP клиент с Chrome профилем
- **bogdanfinn/fhttp** - HTTP библиотека

## ⚙️ Конфигурация

```json
{
  "accounts": [
    {
      "name": "Мой аккаунт",
      "phone_number": "+79123456789",
      "api_id": 123456,
      "api_hash": "your_api_hash_from_my_telegram_org",
      
      "bot_username": "stickersbot",
      "web_app_url": "https://stickers.bot/app", 
      "token_api_url": "https://api.stickers.bot",
      
      "auth_token": ""
    }
  ]
}
```

## 🔍 Логирование

```
🔍 Получение auth data для бота: stickersbot
🔗 Получен Web App URL: https://stickers.bot/app?tgWebAppData=...
🔓 Декодированные auth data: query_id=AAHdF6IqAAAAH0X...
📋 Auth data извлечен, истекает: 15:45:30
✅ Bearer токен получен через API: eyJh****...def4
```

## 🛠️ Преимущества интеграции

1. **Единый HTTP клиент** - используется тот же клиент что и для покупки стикеров
2. **TLS профиль Chrome** - обходит защиты сайтов
3. **Консистентность** - все HTTP запросы через один интерфейс
4. **Поддержка cookies** - автоматическое управление сессиями

## 🎯 Полный процесс

1. **Авторизация в Telegram** → session файл
2. **Получение Auth Data** → tgWebAppData через gotd/td
3. **Декодирование данных** → query_id, user, hash
4. **Отправка на API** → POST /auth через client.HTTPClient
5. **Получение Bearer токена** → eyJhbGc... через tls-client
6. **Использование токена** → Authorization: Bearer ... в BuyStickers

---

**✨ Теперь авторизация полностью интегрирована с существующим HTTP клиентом!** 
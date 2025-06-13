# TelegramMinter - Автоматическая покупка стикеров

Приложение для автоматической покупки стикеров через API StickerDom с автоматической отправкой TON транзакций.

## Возможности

- ✅ Автоматическая покупка стикеров через API
- ✅ Автоматическая отправка TON транзакций на блокчейн
- ✅ Парсинг данных из ответа API (order_id, total_amount, wallet)
- ✅ Тестовый режим для безопасного тестирования
- ✅ Многопоточная обработка
- ✅ Подробная статистика и логирование

## Конфигурация

Создайте файл `config.json` на основе `config.example.json`:

```json
{
  "auth_token": "YOUR_JWT_TOKEN",
  "threads": 2,
  "collection": 4,
  "character": 15,
  "currency": "TON",
  "count": 5,
  "seed_phrase": "your twenty four word seed phrase goes here",
  "test_mode": false,
  "test_address": "UQBfuEnLEUF8JEbXpknjmxGqeZsNR2CX9MIJfZVi99M1OCEF"
}
```

### Параметры конфигурации

- `auth_token` - JWT токен для авторизации в API StickerDom
- `threads` - количество потоков для покупки (рекомендуется 1-3)
- `collection` - ID коллекции стикеров
- `character` - ID персонажа
- `currency` - валюта платежа (обычно "TON")
- `count` - количество стикеров для покупки
- `seed_phrase` - SEED фраза вашего TON кошелька (24 слова через пробел)
- `test_mode` - включить тестовый режим (true/false)
- `test_address` - адрес для отправки в тестовом режиме

## Как работает

1. **Покупка стикеров**: Приложение отправляет POST запрос к API StickerDom
2. **Парсинг ответа**: Извлекает `order_id`, `total_amount`, `currency`, `wallet` из JSON ответа
3. **Отправка TON**: Создает и отправляет TON транзакцию с:
   - Суммой: `total_amount + комиссия (0.25 TON)`
   - Комментарием: `order_id`
   - Получателем: `wallet` (или `test_address` в тестовом режиме)

## Установка и запуск

1. Клонируйте репозиторий
2. Установите зависимости: `go mod tidy`
3. Создайте `config.json` на основе примера
4. Запустите: `go run cmd/stickersbot/main.go`

## Пример успешного ответа API

```json
{
  "ok": true,
  "data": {
    "order_id": "567d37de-895c-4290-b2a0-d338083eddab",
    "total_amount": 25050000000,
    "currency": "TON",
    "wallet": "UQBfuEnLEUF8JEbXpknjmxGqeZsNR2CX9MIJfZVi99M1OCEF"
  }
}
```

## Безопасность

⚠️ **ВАЖНО**: 
- Храните seed фразу в безопасности
- Используйте тестовый режим для проверки работы
- Проверьте баланс кошелька перед запуском
- Seed фраза должна содержать ровно 24 слова

## Логи и статистика

Приложение показывает подробную статистику:
- Общее количество запросов
- Успешные запросы
- Ошибки
- Количество отправленных TON транзакций
- RPS (запросов в секунду)
- Время работы

## Построение

```bash
go build -o bin/stickersbot cmd/stickersbot/main.go
```
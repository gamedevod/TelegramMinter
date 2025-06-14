# Telegram Auto Buy - Automatic Sticker Purchasing Bot

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                                                                              ║
║    ████████╗███████╗██╗     ███████╗ ██████╗ ██████╗  █████╗ ███╗   ███╗    ║
║    ╚══██╔══╝██╔════╝██║     ██╔════╝██╔════╝ ██╔══██╗██╔══██╗████╗ ████║    ║
║       ██║   █████╗  ██║     █████╗  ██║  ███╗██████╔╝███████║██╔████╔██║    ║
║       ██║   ██╔══╝  ██║     ██╔══╝  ██║   ██║██╔══██╗██╔══██║██║╚██╔╝██║    ║
║       ██║   ███████╗███████╗███████╗╚██████╔╝██║  ██║██║  ██║██║ ╚═╝ ██║    ║
║       ╚═╝   ╚══════╝╚══════╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝    ║
║                                                                              ║
║                      █████╗ ██╗   ██╗████████╗ ██████╗                      ║
║                     ██╔══██╗██║   ██║╚══██╔══╝██╔═══██╗                     ║
║                     ███████║██║   ██║   ██║   ██║   ██║                     ║
║                     ██╔══██║██║   ██║   ██║   ██║   ██║                     ║
║                     ██║  ██║╚██████╔╝   ██║   ╚██████╔╝                     ║
║                     ╚═╝  ╚═╝ ╚═════╝    ╚═╝    ╚═════╝                      ║
║                                                                              ║
║                          ██████╗ ██╗   ██╗██╗   ██╗                         ║
║                          ██╔══██╗██║   ██║╚██╗ ██╔╝                         ║
║                          ██████╔╝██║   ██║ ╚████╔╝                          ║
║                          ██╔══██╗██║   ██║  ╚██╔╝                           ║
║                          ██████╔╝╚██████╔╝   ██║                            ║
║                          ╚═════╝  ╚═════╝    ╚═╝                            ║
║                                                                              ║
║                                                                              ║
║  ⚓ Developed by: DUO ON DECK Team                                           ║
║  🚀 Project: Telegram Auto Buy                                              ║
║  📧 Support: @black_beard68                                                 ║
║  📢 Channel: @two_on_deck                                                   ║
║  🌊 "Two minds, one mission - sailing the crypto seas!"                     ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

## 🎯 What is this?

Telegram Auto Buy is a program for automatic sticker purchasing in Telegram. The program can:

> 📢 **Stay updated!** Join our official channel [@two_on_deck](https://t.me/two_on_deck) for latest updates, tips and life-changing opportunities!

- **Buy stickers automatically** - set it up once, the program works by itself
- **Work with multiple accounts** - you can add many Telegram accounts
- **Send TON payments** - automatically pays for purchases with TON cryptocurrency
- **Track new collections** - finds new stickers and buys them immediately (snipe mode)

## 🚀 Quick Start

### Step 1: Download the program
1. Download the program archive (contains `stickersbot.exe` and `config_empty.json`)
2. Extract the archive to any folder on your computer (e.g., `TelegramMinter`)

### Step 2: Configure the configuration file
1. Open the `config.json` file in any text editor
2. Fill in all necessary fields according to the instructions below

### Step 3: Get Telegram API keys

#### How to get API ID and API Hash:
1. Open https://my.telegram.org/auth
2. Log in with your phone number
3. Go to "API development tools"
4. Fill out the form:
   - **App title**: any name (e.g., "MyBot")
   - **Short name**: short name (e.g., "mybot")
   - **URL**: can be left empty
   - **Platform**: select "Desktop"
   - **Description**: any description
5. Click "Create application"
6. **IMPORTANT**: Copy and save:
   - **API ID** (number, e.g., 1234567)
   - **API Hash** (string, e.g., "abcd1234efgh5678ijkl9012mnop3456")

### Step 4: Fill in config.json

In the `config.json` file you will see an empty template. Fill it with your data:

```json
{
  "api_id": 0,
  "api_hash": "",
  "bot_username": "",
  "web_app_url": "",
  "token_api_url": "",
  "test_mode": true,
  "test_address": "",
  "accounts": [
    {
      "name": "",
      "phone_number": "",
      "collection": 0,
      "character": 0,
      "currency": "TON",
      "count": 1,
      "threads": 1,
      "max_transactions": 0,
      "seed_phrase": ""
    }
  ]
}
```

**Example of filled config:**
```json
{
  "api_id": 1234567,
  "api_hash": "abcd1234efgh5678ijkl9012mnop3456",
  "bot_username": "mystickersbot",
  "web_app_url": "https://t.me/mystickersbot/app",
  "token_api_url": "https://api.example.com/token",
  "test_mode": true,
  "test_address": "UQD...",
  "accounts": [
    {
      "name": "My Main Account",
      "phone_number": "+1234567890",
      "collection": 123,
      "character": 456,
      "currency": "TON",
      "count": 1,
      "threads": 1,
      "max_transactions": 10,
      "seed_phrase": "word1 word2 word3 ... word24"
    }
  ]
}
```

## ⚙️ Detailed Configuration Description

### Main settings:

- **`api_id`** - Your Telegram application ID (obtained in step 3)
- **`api_hash`** - Your Telegram application hash (obtained in step 3)
- **`bot_username`** - Bot name without @ symbol (e.g., if bot is @mystickersbot, write "mystickersbot")
- **`web_app_url`** - Link to the bot's web application
- **`token_api_url`** - API link for getting tokens
- **`test_mode`** - Test mode (true = test, false = real purchases)
- **`test_address`** - Wallet address for test payments

### Account settings:

- **`name`** - Any name for the account (for convenience)
- **`phone_number`** - Telegram account phone number (with country code, e.g., "+1234567890")
- **`collection`** - Sticker collection ID for purchase
- **`character`** - Character ID in the collection
- **`currency`** - Currency for purchase ("TON", "USDT", etc.)
- **`count`** - Number of stickers to buy at once
- **`threads`** - Number of threads (recommended 1-3)
- **`max_transactions`** - Maximum transactions (0 = no limit)
- **`seed_phrase`** - TON wallet seed phrase (12-24 words separated by spaces)

### Snipe mode (optional):

The program has two operating modes depending on the `snipe_monitor` setting:

#### 🎯 **Snipe Monitor Mode** (`"enabled": true`)
If you want to automatically monitor and buy NEW collections as they appear, add to account:

```json
"snipe_monitor": {
  "enabled": true,
  "supply_range": {
    "min": 1,
    "max": 1000
  },
  "price_range": {
    "min": 1000000000,
    "max": 10000000000
  },
  "word_filter": ["possible pack names", "possible pack names 2"]
}
```

**How it works:**
- The program will **monitor** for new collections that match your criteria
- When a new collection appears, it will automatically try to mint it
- The `collection` and `character` IDs in the account settings are **ignored**
- Perfect for catching new drops and rare collections

#### 🎯 **Direct Mint Mode** (`"enabled": false` or no snipe_monitor section)
If you want to mint specific collections immediately:

```json
{
  "name": "My Account",
  "collection": 123,
  "character": 456,
  // ... other settings
  // No snipe_monitor section or "enabled": false
}
```

**How it works:**
- The program will **immediately** try to mint the specified `collection` and `character` IDs
- No monitoring - direct action
- Perfect when you know exactly what you want to mint

#### Snipe Monitor Settings:
- **`enabled`** - Enable snipe mode (true = monitor mode, false = direct mint mode)
- **`supply_range`** - Range of sticker quantity (min-max)
- **`price_range`** - Price range in nanotons (1 TON = 1000000000 nanotons)
- **`word_filter`** - List of words to search for in collection names

## 🏃‍♂️ How to Run

### Windows:
1. Open command prompt (Win+R, type `cmd`)
2. Navigate to the program folder: `cd C:\path\to\folder\TelegramMinter`
3. Run: `stickersbot.exe`

### First run:
1. The program will ask for a confirmation code from Telegram
2. Enter the code that comes to Telegram
3. If the account has two-factor authentication, enter the password
4. The program will save the session and won't ask for codes again

## 📊 What Program Messages Mean

- **🚀 Sticker purchasing started!** - Program started
- **📈 Total: X | Success: Y | Errors: Z** - Statistics (total requests, successful, errors)
- **💰 Transaction sent!** - TON transaction sent
- **🔑 Invalid auth token!** - Authorization token expired (program will update automatically)
- **🎯 New collection found** - New collection found (in snipe mode)

## ❗ Important Notes

### Security:
- **DO NOT SHARE** your API ID, API Hash, and seed phrases
- Keep the `config.json` file in a safe place
- Use `test_mode: true` for testing

### TON wallet seed phrase:
- These are 12-24 words from your TON wallet
- Needed for automatic payment sending
- Can be obtained from TON Wallet or Tonkeeper app
- **IMPORTANT**: Seed phrase gives full access to the wallet!

### Limits:
- Don't set too many threads - you might get banned
- Use `max_transactions` to limit expenses
- In `test_mode` no money is spent

## 🆘 Troubleshooting

### "Configuration loading error":
- Check the syntax of `config.json` file
- Make sure all quotes and commas are in place

### "Authorization error":
- Check the correctness of `api_id` and `api_hash`
- Make sure the phone number is specified with country code

### "Invalid auth token":
- This is normal, the program will update the token automatically
- If the error repeats, check bot settings

### Program doesn't buy:
- Check that `test_mode: false` for real purchases
- Make sure there's TON in the wallet
- Check the correctness of `collection` and `character` IDs

## 📞 Support

If you encounter problems:
1. Check all settings in `config.json`
2. Make sure you're using the correct API keys
3. Try in test mode first
4. Check program logs for errors

**Need help?** Contact our support team:
- 📧 Support: [@black_beard68](https://t.me/black_beard68)
- 📢 Channel: [@two_on_deck](https://t.me/two_on_deck)
- ⚓ Team: **DUO ON DECK** - "Two minds, one mission!"

---

**⚠️ WARNING**: This program works with real money (TON). Always test in `test_mode` first before using in production!

---
*Developed with ❤️ by DUO ON DECK Team*  
*🌊 Sailing the crypto seas since 2024* 
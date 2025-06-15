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
1. Download the program archive (contains `stickersbot.exe` and `config.json`)
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
  "license_key": "",
  "api_id": 0,
  "api_hash": "",
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
      "seed_phrase": "",
      "snipe_monitor": {
        "enabled": false,
        "supply_range": {
          "min": 10,
          "max": 5000
        },
        "price_range": {
          "min": 1000000000,
          "max": 10000000000
        },
        "word_filter": ["keyword1", "keyword2"]
      }
    }
  ]
}
```

**Example of filled config:**
```json
{
  "license_key": "YOUR_LICENSE_KEY_HERE",
  "api_id": 1234567,
  "api_hash": "abcd1234efgh5678ijkl9012mnop3456",
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
      "seed_phrase": "word1 word2 word3 ... word24",
      "snipe_monitor": {
        "enabled": false,
        "supply_range": {
          "min": 10,
          "max": 5000
        },
        "price_range": {
          "min": 1000000000,
          "max": 10000000000
        },
        "word_filter": ["azuki", "pokemon", "cryptopunks"]
      }
    }
  ]
}
```

## ⚙️ Detailed Configuration Description

### Main settings:

- **`license_key`** - Program license key (obtain from developers)
- **`api_id`** - Your Telegram application ID (obtained in step 3)
- **`api_hash`** - Your Telegram application hash (obtained in step 3)
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
- **`snipe_monitor`** - Snipe monitoring settings (optional)

## ⚙️ Operating Modes

> **⚠️ IMPORTANT:** The program has two fundamentally different operating modes depending on the `snipe_monitor` setting!

### 🔍 Snipe Monitoring Mode (`snipe_monitor.enabled: true`)

**How it works:**
- The program **waits for NEW collections** to appear on the platform
- When a new collection appears that matches your criteria, the program **automatically buys it**
- The `collection` and `character` parameters in the config are **IGNORED**
- The program works in **constant monitoring** mode

**Example config with snipe monitoring enabled:**
```json
{
  "name": "Sniper Account",
  "phone_number": "+1234567890",
  "collection": 0,  // ← Ignored in this mode
  "character": 0,   // ← Ignored in this mode
  "currency": "TON",
  "count": 1,
  "threads": 1,
  "max_transactions": 5,
  "seed_phrase": "your seed phrase here",
  "snipe_monitor": {
    "enabled": true,  // ← SNIPE MODE ENABLED
    "supply_range": {
      "min": 100,
      "max": 10000
    },
    "price_range": {
      "min": 500000000,
      "max": 5000000000
    },
    "word_filter": ["rare", "limited", "exclusive"]
  }
}
```

### 🎯 Direct Purchase Mode (`snipe_monitor.enabled: false` or absent)

**How it works:**
- The program **immediately buys** the specified `collection` and `character` from the config
- No monitoring - **direct action**
- The `collection` and `character` parameters are **REQUIRED**
- Perfect when you **know exactly** what you want to buy

**Example config with direct purchase:**
```json
{
  "name": "Direct Purchase",
  "phone_number": "+1234567890",
  "collection": 123,  // ← REQUIRED collection ID
  "character": 456,   // ← REQUIRED character ID
  "currency": "TON",
  "count": 1,
  "threads": 1,
  "max_transactions": 1,
  "seed_phrase": "your seed phrase here",
  "snipe_monitor": {
    "enabled": false  // ← SNIPE MODE DISABLED
  }
}
```

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

## 🎮 Application Menu Guide

After launching the program, you will see an interactive CLI menu with 5 main options. Here's a detailed guide for each menu item:

### 🚀 1. Start Task (Purchase/Monitoring)

**What it does:**
- Starts the main bot functionality
- Authenticates all configured accounts
- Begins purchasing or monitoring based on your config
- Shows real-time logs and statistics

**Step-by-step process:**
1. **Authentication:** Bot checks all accounts and authenticates those that need it
2. **Validation:** Verifies wallet balances and configuration
3. **Start:** Begins the purchasing/monitoring process
4. **Monitoring:** Shows live logs and statistics in the console

> ⚠️ **Important:** Make sure all accounts are properly configured before starting!

### 🛑 2. Stop Task

**What it does:**
- Gracefully stops all running tasks
- Completes current transactions before stopping
- Shows final statistics
- Saves all data and logs

**When to use:**
- When you want to stop the bot temporarily
- Before changing configuration
- When you've reached your purchase goals
- If you notice any issues and want to investigate

### 🔐 3. Manage Account Authentication

**What it does:**
- Shows authentication status of all accounts
- Allows selective authentication of specific accounts
- Displays session files and auth tokens status
- Provides detailed account information

**Account Status Information:**
- 🟢 **ACTIVE** - Account has valid auth token or session file
- 🔴 **INACTIVE** - Account needs authentication
- ✅ **Auth Token** - Account has valid bearer token
- 📁 **Session** - Telegram session file exists

**Authentication Options:**
1. **Selective Authentication:** Choose specific accounts by numbers (e.g., 1,3,5)
2. **Authenticate All:** Authenticate all inactive accounts at once
3. **Refresh Status:** Update account status information

> ⚠️ **Note:** Authentication may require phone verification codes for new accounts.

### 💰 4. Show Wallet Balances

**What it does:**
- Displays TON wallet balances for all accounts
- Shows wallet addresses
- Indicates any wallet-related errors
- Helps verify sufficient funds before purchasing

**Information displayed:**
- **Account Name:** Your configured account name
- **Phone Number:** Masked phone number for security
- **Wallet Address:** TON wallet address
- **Balance:** Current TON balance
- **Errors:** Any wallet-related issues

> 💡 **Tip:** Check balances regularly to ensure you have enough TON for purchases!

### 🚪 5. Exit

**What it does:**
- Safely closes the application
- Stops all running tasks
- Saves current state
- Performs cleanup operations

**Best practices:**
- Always use this option to exit (don't force close)
- Wait for tasks to complete before exiting
- Check final statistics before closing

### 📊 Understanding Status Messages

| Status | Meaning |
|--------|---------|
| ⭕ Stopped | No tasks are currently running |
| 🟢 Running | Bot is actively purchasing/monitoring |
| 🔄 Authenticating | Account authentication in progress |
| ✅ Success | Operation completed successfully |
| ❌ Error | Operation failed - check error message |

### 🔧 Troubleshooting Common Issues

**Account Authentication Problems:**
- **Session not found:** Use menu option 3 to authenticate
- **Invalid phone number:** Ensure phone starts with '+' and country code
- **2FA required:** Add `two_factor_password` to config

**Wallet Issues:**
- **Insufficient balance:** Add more TON to your wallet
- **Invalid seed phrase:** Check seed phrase format (12-24 words)
- **Wallet not found:** Verify seed phrase is correct

**Purchase Problems:**
- **Collection not found:** Verify collection and character IDs
- **Price changed:** Collection price may have been updated
- **Sold out:** Collection may be out of stock

> 💡 **Pro Tips:**
> - Always test with small amounts first
> - Keep backup copies of your config file
> - Monitor logs for any error messages
> - Use test mode for initial setup

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
- **DO NOT SHARE** your license key, API ID, API Hash, and seed phrases
- Keep the `config.json` file in a safe place
- Use `test_mode: true` for testing
- **License key** is required for the program to work in production mode

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

### "License key is not specified":
- Make sure the `license_key` field is filled in `config.json`
- Obtain the license key from developers

### "License authentication failed":
- Check the correctness of the license key
- Make sure you have internet access
- Contact support to check license status

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
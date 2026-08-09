# Help Trenchers

An advanced Solana token monitoring and analysis toolkit for tracking token migrations, developer activity, Twitter/X signals, and launchpad data.

## Features

### Migration Scanner

- Monitors tokens on Pump.fun, Bonk, Boop, and LaunchLab.
- Calculates migration statistics based on configurable parameters.
- Automatically opens tokens that match the selected filters.

### Migration Sniper

- Detects token migrations in real time.
- Filters migrations according to user-defined conditions.
- Supports automatic opening of matching tokens.

### Twitter/X Community Checker

- Detects tokens associated with Twitter/X Communities.
- Displays the Community administrator.
- Shows the administrator's follower count.
- Supports automatic account opening using usernames from a file.

### Advanced Twitter/X Tracker

- Tracks selected Twitter/X accounts.
- Displays follower counts and account information.
- Supports automatic opening of accounts from a whitelist.

### LaunchCoin Alerts

- Monitors tokens launched on LaunchCoin.
- Displays the token developer.
- Shows the developer's follower count and Twitter/X profile link.
- Supports automatic opening when the developer reaches a specified follower threshold.

### Twitter/X Contract Address Sniper

- Scans Twitter/X for contract addresses.
- Supports Bonk, Boop, Pump.fun, and `CA:` formats.
- Automatically opens detected contract addresses.

### Developer Wallet Sniper

- Tracks developers by wallet address.
- Detects tokens associated with specific developer wallets.
- Helps analyze developer activity across multiple launches.

### Community Admin Statistics

- Analyzes tokens created by Twitter/X Community administrators.
- Calculates average market capitalization.
- Calculates median market capitalization.
- Displays the three most recent tokens and their market caps.

## Supported Platforms

- Pump.fun
- Bonk
- Boop
- LaunchLab
- LaunchCoin
- Twitter/X

## Configuration

The application supports configurable filters, including:

- Minimum and maximum market capitalization.
- Migration percentage.
- Developer follower count.
- Wallet addresses.
- Twitter/X usernames.
- Whitelisted accounts.
- Automatic token and account opening.

## Disclaimer

This project is intended for research, monitoring, and educational purposes only. It does not provide financial advice. Cryptocurrency trading involves substantial risk, and users are responsible for their own decisions.

# Help Trenchers - Telegram Bot

Бот продажи доступа к софту по подписке. Оплата: ⭐ Telegram Stars или 🪙 Solana (SOL).

## Возможности

- Меню: ℹ️ Информация / 💳 Подписка / 👤 Профиль
- Тарифы: 7 / 14 / 31 дней (2 / 4 / 8 SOL или 14000 / 28000 / 36000 ⭐)
- Уникальный Solana-адрес на каждый заказ; бот сам проверяет поступление и выдаёт ключ
- Авто-вывод полученных SOL на основной кошелёк (`SOL_WALLET`)
- Профиль показывает активный ключ и сколько дней осталось

## Локальный запуск

```bash
cp .env.example .env       
docker compose up -d postgres
go run ./bot/cmd
```

БД (PostgreSQL) должна быть доступна по `DB_URL`. Схема в `bot/db/schema.sql`
применяется вручную: `psql -f bot/db/schema.sql`.

## Переменные окружения

| Переменная     | Назначение                                      |
|----------------|-------------------------------------------------|
| `BOT_TOKEN`    | Токен Telegram-бота от @BotFather               |
| `DB_URL`       | Строка подключения к PostgreSQL                  |
| `SOL_WALLET`   | Твой основной Solana-кошелёк (куда выводятся SOL) |
| `SOL_RPC_URL`  | (опц.) RPC Solana, по умолчанию mainnet-beta    |
| `LOG_LEVEL`    | `development` для подробных логов               |

## Деплой (GitHub Actions → VPS)

1. `git push` в ветку `main` собирает Docker-образ и пушит в GHCR.
2. В репозитории GitHub задай Secrets:
   - `SSH_HOST`, `SSH_USER`, `SSH_PRIVATE_KEY`, `SSH_PORT`
   - `BOT_TOKEN`, `SOL_WALLET`, `DB_URL` (используются на сервере)
3. На сервере создай `/opt/help-trenchers`, положи туда `docker-compose.yaml` и `.env`.
   В `.env` можно задать `BOT_IMAGE=ghcr.io/<user>/<repo>:<tag>`, тогда сервер
   будет тянуть готовый образ из GHCR вместо локальной сборки.
4. Пуш в `main` задеплоит бот через SSH (`docker compose up -d`).

## Безопасность

- Приватные ключи временных Solana-адресов хранятся в БД (`orders.deposit_privkey`).
  БД должна быть защищена; при компрометации можно вывести только уже пришедшие
  на временные адреса средства.
- `.env` не коммитится (в `.gitignore`).

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

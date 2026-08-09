# Help Trenchers — Telegram Bot

Бот продажи доступа к софту по подписке. Оплата: ⭐ Telegram Stars или 🪙 Solana (SOL).

## Возможности

- Меню: ℹ️ Информация / 💳 Подписка / 👤 Профиль
- Тарифы: 7 / 14 / 31 дней (2 / 4 / 8 SOL или 14000 / 28000 / 36000 ⭐)
- Уникальный Solana-адрес на каждый заказ; бот сам проверяет поступление и выдаёт ключ
- Авто-вывод полученных SOL на основной кошелёк (`SOL_WALLET`)
- Профиль показывает активный ключ и сколько дней осталось

## Локальный запуск

```bash
cp .env.example .env        # заполни BOT_TOKEN, DB_URL, SOL_WALLET
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
4. Пуш в `main` задеплоит бот через SSH (`docker compose up -d`).

## Безопасность

- Приватные ключи временных Solana-адресов хранятся в БД (`orders.deposit_privkey`).
  БД должна быть защищена; при компрометации можно вывести только уже пришедшие
  на временные адреса средства.
- `.env` не коммитится (в `.gitignore`).

## License

MIT. См. `LICENSE`.

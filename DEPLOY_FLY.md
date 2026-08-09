# Деплой на Fly.io (бесплатно)

Fly.io даёт бесплатный VM (256MB, shared CPU) с постоянным доменом
`https://<app>.fly.dev` и авто-HTTPS. Cloudflare tunnel в этом случае **не нужен** —
IPN и десктоп стучатся прямо на `*.fly.dev`.

## 1. Подготовка

```bash
# поставить flyctl (Linux)
curl -L https://fly.io/install.sh | sh
export PATH="$HOME/.fly/bin:$PATH"

# вход (откроется браузер / код)
fly auth login
```

## 2. База данных (PostgreSQL)

Fly free VM не включает БД. Возьми бесплатную внешнюю Postgres:

- **Neon** (neon.tech) — free tier, serverless, без карты в большинстве случаев.
- **Supabase** (supabase.com) — free tier, 500MB.

Создай БД, возьми строку подключения вида:
```
postgres://user:pass@host:5432/dbname?sslmode=require
```
И накати схему + миграцию:
```bash
psql "<DB_URL>" -f bot/db/schema.sql
psql "<DB_URL>" -f bot/db/migrations/0002_nowpayments.sql
```

> Можно и `fly postgres create`, но это отдельная VM (ресурсы). Для старта
> проще внешняя бесплатная БД.

## 3. Секреты

Не клади ключи в fly.toml. Задай их через `fly secrets set` (шифруются, попадают
в окружение контейнера):

```bash
fly secrets set \
  BOT_TOKEN="твой_токен_от_BotFather" \
  DB_URL="postgres://user:pass@host:5432/dbname?sslmode=require" \
  NOWPAYMENTS_API_KEY="CK0E1QD-..." \
  NOWPAYMENTS_IPN_SECRET="SbIJ9HKD..." \
  SOL_WALLET="" \
  SOL_RPC_URL="https://api.mainnet-beta.solana.com"
```

После деплоя узнаешь домен и добавишь IPN URL:
```bash
fly secrets set NOWPAYMENTS_IPN_URL="https://help-trenchers.fly.dev/api/nowpayments/callback"
```
(имя приложения — из `fly.toml`, `app = "help-trenchers"`; если заменишь — поправь URL).

## 4. Деплой

Первый раз (создаёт приложение по fly.toml):
```bash
fly launch --no-deploy   # если ещё не создано
fly deploy
```

Дальше только:
```bash
fly deploy
```

## 5. Проверка

```bash
fly logs                 # логи бота
fly status               # состояние машины
curl https://help-trenchers.fly.dev/api/health   # должно вернуть "ok"
```

В кабинете NOWPayments → Settings → IPN укажи:
`https://help-trenchers.fly.dev/api/nowpayments/callback`

## 6. Переменные (итог)

| Переменная              | Как задаётся                  |
|-------------------------|-------------------------------|
| `BOT_TOKEN`             | `fly secrets set`             |
| `DB_URL`                | `fly secrets set`             |
| `NOWPAYMENTS_API_KEY`   | `fly secrets set`             |
| `NOWPAYMENTS_IPN_SECRET`| `fly secrets set`             |
| `NOWPAYMENTS_IPN_URL`   | `fly secrets set` (домен fly)|
| `API_ADDR`             | в `fly.toml` (`":8080"`)     |
| `SOL_WALLET`            | `fly secrets set` (опц.)     |

Бот слушает `:8080` внутри контейнера; Fly выводит его наружу с HTTPS.
`auto_stop_machines = "off"` в fly.toml — чтобы бот не засыпал и ловил IPN/опрос
оплат.

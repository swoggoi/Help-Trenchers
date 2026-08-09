-- Миграция: добавляем поле np_invoice_id в orders для NowPayments.
ALTER TABLE IF EXISTS orders
    ADD COLUMN IF NOT EXISTS np_invoice_id TEXT;

ALTER TABLE raffles
    ADD COLUMN raffle_token VARCHAR;

UPDATE raffles
SET raffle_token = md5(raffle_id::text || ':' || random()::text || ':' || clock_timestamp()::text)
WHERE raffle_token IS NULL;

ALTER TABLE raffles
    ALTER COLUMN raffle_token SET NOT NULL;

ALTER TABLE raffles
    ADD CONSTRAINT raffles_raffle_token_key UNIQUE (raffle_token);

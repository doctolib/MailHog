SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE TABLE IF NOT EXISTS messages (
    message jsonb
);

-- messages has no primary key, so the default replica identity (which requires
-- one) can't cover DELETEs. Namespaces with a FOR ALL TABLES CDC publication
-- enabled (a platform-wide Terraform default) reject DELETE FROM messages
-- without this, regardless of this app's own cdc.enabled setting.
ALTER TABLE messages REPLICA IDENTITY FULL;

CREATE INDEX IF NOT EXISTS messages_expr_idx ON messages USING btree (((message -> 'Created'::text)));
CREATE INDEX IF NOT EXISTS messages_expr_idx2 ON messages USING btree (((message ->> 'ID'::text)));

-- TODO: can we make this indexes better by using "Content" (parsed body) rather than "Raw" (SMTP wire protocol)?
CREATE INDEX IF NOT EXISTS messages_expr_idx3 ON messages USING gin (to_tsvector('english'::regconfig, ((message -> 'Raw'::text) ->> 'Data'::text)));
CREATE INDEX IF NOT EXISTS messages_expr_idx4 ON messages USING gin (to_tsvector('english'::regconfig, ((message -> 'Raw'::text) ->> 'From'::text)));
CREATE INDEX IF NOT EXISTS messages_expr_idx5 ON messages USING gin (to_tsvector('english'::regconfig, ((message -> 'Raw'::text) ->> 'To'::text)));

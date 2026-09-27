CREATE SCHEMA IF NOT EXISTS notify;

CREATE TABLE notify.searches (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL,
  name text NOT NULL,
  neighborhood_id uuid NOT NULL,
  transaction_type text NOT NULL DEFAULT '',
  locale text NOT NULL CHECK (locale IN ('ar', 'en')),
  channel text NOT NULL DEFAULT 'in_app' CHECK (channel = 'in_app'),
  muted boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notify_searches_user ON notify.searches (user_id, created_at);

CREATE TABLE notify.mutes (
  user_id uuid NOT NULL,
  channel text NOT NULL CHECK (channel = 'in_app'),
  muted boolean NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, channel)
);

CREATE TABLE notify.events (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('listing_match', 'visit_change', 'payment_review')),
  entity_id uuid NOT NULL,
  search_id uuid,
  dedup_key text NOT NULL UNIQUE,
  title_ar text NOT NULL,
  title_en text NOT NULL,
  body_ar text NOT NULL,
  body_en text NOT NULL,
  channel text NOT NULL DEFAULT 'in_app' CHECK (channel = 'in_app'),
  delivery_status text NOT NULL DEFAULT 'not_sent' CHECK (delivery_status = 'not_sent'),
  provider_adopted boolean NOT NULL DEFAULT false CHECK (provider_adopted = false),
  suppressed boolean NOT NULL DEFAULT false,
  suppress_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notify_events_user ON notify.events (user_id, created_at DESC);

CREATE SCHEMA IF NOT EXISTS clients;
CREATE SCHEMA IF NOT EXISTS visits;
CREATE SCHEMA IF NOT EXISTS deals;

CREATE TABLE clients.profiles (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  user_id uuid,
  display_name text NOT NULL,
  email text NOT NULL DEFAULT '',
  phone text NOT NULL DEFAULT '',
  source text NOT NULL DEFAULT 'request',
  status text NOT NULL CHECK (status IN ('potential','active')),
  version int NOT NULL DEFAULT 1,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX clients_profile_email ON clients.profiles (lower(email)) WHERE email <> '';

CREATE TABLE clients.requests (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  branch_id uuid,
  profile_id uuid NOT NULL,
  neighborhood_id uuid,
  transaction_type text NOT NULL CHECK (transaction_type IN ('sale','annual','monthly','daily')),
  budget_minor bigint,
  currency text NOT NULL DEFAULT 'SAR',
  timing_note text NOT NULL DEFAULT '',
  specs_ar text NOT NULL DEFAULT '',
  specs_en text NOT NULL DEFAULT '',
  contact_method text NOT NULL CHECK (contact_method IN ('phone','email','either')),
  stage text NOT NULL,
  assignee_id uuid,
  close_reason text NOT NULL DEFAULT '',
  deal_id uuid,
  version int NOT NULL DEFAULT 1,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX clients_requests_stage ON clients.requests (company_id, stage, created_at DESC);

CREATE TABLE visits.appointments (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  branch_id uuid,
  profile_id uuid NOT NULL,
  listing_id uuid,
  property_id uuid,
  unit_id uuid,
  broker_user_id uuid,
  host_user_id uuid,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  attendees int NOT NULL DEFAULT 1,
  status text NOT NULL,
  broker_confirmed_at timestamptz,
  owner_confirmed_at timestamptz,
  result text NOT NULL DEFAULT '',
  result_note text NOT NULL DEFAULT '',
  instructions_ar text NOT NULL DEFAULT '',
  instructions_en text NOT NULL DEFAULT '',
  cancel_reason text NOT NULL DEFAULT '',
  escalate_at timestamptz,
  version int NOT NULL DEFAULT 1,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX visits_broker_time ON visits.appointments (broker_user_id, starts_at);
CREATE INDEX visits_status ON visits.appointments (company_id, status, starts_at);

CREATE TABLE visits.history (
  id uuid PRIMARY KEY,
  visit_id uuid NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  previous_status text NOT NULL,
  reason text NOT NULL DEFAULT '',
  actor_id uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE visits.reminders (
  id uuid PRIMARY KEY,
  visit_id uuid NOT NULL,
  kind text NOT NULL,
  status text NOT NULL CHECK (status IN ('recorded','cancelled')),
  fire_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE visits.completion_counter (
  id text PRIMARY KEY,
  total int NOT NULL DEFAULT 0
);

INSERT INTO visits.completion_counter (id, total) VALUES ('shamal', 0);

CREATE TABLE deals.deals (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  branch_id uuid,
  request_id uuid,
  profile_id uuid NOT NULL,
  listing_id uuid,
  property_id uuid,
  neighborhood_id uuid,
  transaction_type text NOT NULL CHECK (transaction_type IN ('sale','annual','monthly','daily')),
  broker_user_id uuid,
  owner_user_id uuid,
  status text NOT NULL,
  amount_minor bigint,
  currency text NOT NULL DEFAULT 'SAR',
  agreement_ref text NOT NULL DEFAULT '',
  close_date date,
  close_requested_by uuid,
  loss_reason text NOT NULL DEFAULT '',
  is_test boolean NOT NULL DEFAULT false,
  version int NOT NULL DEFAULT 1,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX deals_status ON deals.deals (company_id, status, updated_at DESC);

CREATE TABLE deals.stage_events (
  id uuid PRIMARY KEY,
  deal_id uuid NOT NULL,
  from_status text NOT NULL,
  to_status text NOT NULL,
  reason text NOT NULL DEFAULT '',
  actor_id uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

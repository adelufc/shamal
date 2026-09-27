CREATE SCHEMA IF NOT EXISTS finance;

CREATE SEQUENCE finance.invoice_serial START 1000;

CREATE TABLE finance.attempts (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL,
  actor_id uuid NOT NULL,
  company_id uuid NOT NULL,
  method text NOT NULL CHECK (method IN ('mada','visa','mastercard','bank_transfer')),
  provider text NOT NULL CHECK (provider = 'test_adapter'),
  amount_minor bigint NOT NULL CHECK (amount_minor > 0),
  currency text NOT NULL CHECK (currency = 'SAR'),
  status text NOT NULL CHECK (status IN ('pending_review','mismatch_review','collected')),
  gateway_adopted boolean NOT NULL DEFAULT false CHECK (gateway_adopted = false),
  review_deadline timestamptz,
  idempotency_key text NOT NULL,
  request_hash text NOT NULL,
  proof_note text NOT NULL DEFAULT '',
  proof_confirms boolean NOT NULL DEFAULT false CHECK (proof_confirms = false),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (actor_id, idempotency_key)
);

CREATE TABLE finance.provider_events (
  id text PRIMARY KEY,
  attempt_id uuid NOT NULL,
  amount_minor bigint NOT NULL,
  currency text NOT NULL,
  outcome text NOT NULL CHECK (outcome IN ('captured','failed','pending')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE finance.journals (
  id uuid PRIMARY KEY,
  kind text NOT NULL CHECK (kind IN ('invoice','collection','reversal')),
  stay_id uuid NOT NULL,
  attempt_id uuid,
  invoice_id uuid,
  event_id text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX finance_journal_event ON finance.journals (event_id) WHERE event_id IS NOT NULL;

CREATE TABLE finance.ledger_lines (
  id uuid PRIMARY KEY,
  journal_id uuid NOT NULL,
  account text NOT NULL CHECK (account IN (
    'receivable','stay_revenue','provider_clearing','provider_fee','commission','tax','deposit_liability','refund','landlord_payable'
  )),
  direction text NOT NULL CHECK (direction IN ('debit','credit')),
  amount_minor bigint NOT NULL CHECK (amount_minor > 0),
  currency text NOT NULL CHECK (currency = 'SAR'),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE finance.invoices (
  id uuid PRIMARY KEY,
  serial bigint NOT NULL UNIQUE,
  stay_id uuid NOT NULL UNIQUE,
  attempt_id uuid,
  actor_id uuid NOT NULL,
  company_id uuid NOT NULL,
  currency text NOT NULL CHECK (currency = 'SAR'),
  total_minor bigint NOT NULL CHECK (total_minor > 0),
  seller_adopted boolean NOT NULL DEFAULT false CHECK (seller_adopted = false),
  issued_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE finance.invoice_lines (
  id uuid PRIMARY KEY,
  invoice_id uuid NOT NULL,
  code text NOT NULL,
  quantity int NOT NULL CHECK (quantity > 0),
  unit_minor bigint NOT NULL,
  amount_minor bigint NOT NULL,
  counts_as_stay boolean NOT NULL
);

CREATE TABLE finance.corrections (
  id uuid PRIMARY KEY,
  invoice_id uuid NOT NULL,
  actor_id uuid NOT NULL,
  reason text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE finance.receipts (
  id uuid PRIMARY KEY,
  attempt_id uuid NOT NULL UNIQUE,
  invoice_id uuid,
  amount_minor bigint NOT NULL,
  currency text NOT NULL CHECK (currency = 'SAR'),
  event_id text NOT NULL UNIQUE,
  issued_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE finance.refunds (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL,
  requester_id uuid NOT NULL,
  amount_minor bigint NOT NULL CHECK (amount_minor > 0),
  currency text NOT NULL CHECK (currency = 'SAR'),
  reason text NOT NULL,
  status text NOT NULL CHECK (status IN (
    'under_review','approved','sent_to_provider','successful','rejected','failed_retryable','cancelled'
  )),
  idempotency_key text NOT NULL,
  request_hash text NOT NULL,
  first_approver uuid,
  second_approver uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (requester_id, idempotency_key)
);

CREATE TABLE finance.stay_locks (
  stay_id uuid PRIMARY KEY
);

CREATE OR REPLACE FUNCTION finance.reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'append_only' USING ERRCODE = '55000';
END $$;

CREATE TRIGGER journals_append_only BEFORE UPDATE OR DELETE ON finance.journals
FOR EACH ROW EXECUTE FUNCTION finance.reject_change();

CREATE TRIGGER ledger_append_only BEFORE UPDATE OR DELETE ON finance.ledger_lines
FOR EACH ROW EXECUTE FUNCTION finance.reject_change();

CREATE TRIGGER invoices_append_only BEFORE UPDATE OR DELETE ON finance.invoices
FOR EACH ROW EXECUTE FUNCTION finance.reject_change();

CREATE TRIGGER invoice_lines_append_only BEFORE UPDATE OR DELETE ON finance.invoice_lines
FOR EACH ROW EXECUTE FUNCTION finance.reject_change();

CREATE TRIGGER receipts_append_only BEFORE UPDATE OR DELETE ON finance.receipts
FOR EACH ROW EXECUTE FUNCTION finance.reject_change();

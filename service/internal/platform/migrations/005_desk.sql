CREATE SCHEMA IF NOT EXISTS messages;
CREATE SCHEMA IF NOT EXISTS disputes;

CREATE TABLE messages.conversations (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  subject_kind text NOT NULL CHECK (subject_kind IN ('listing','visit','booking','deal','dispute')),
  subject_id uuid NOT NULL,
  assignee_id uuid,
  review_status text NOT NULL DEFAULT 'open' CHECK (review_status IN ('open','blocked_review','abuse_review')),
  recipient_locale text NOT NULL CHECK (recipient_locale IN ('ar','en')),
  first_human_reply_at timestamptz,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (subject_kind, subject_id, created_by)
);

CREATE TABLE messages.participants (
  conversation_id uuid NOT NULL,
  user_id uuid NOT NULL,
  role text NOT NULL CHECK (role IN ('client','staff')),
  added_by uuid NOT NULL,
  added_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE messages.messages (
  id uuid PRIMARY KEY,
  conversation_id uuid NOT NULL,
  sender_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('message','correction','internal_note','canned')),
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
  corrects_id uuid,
  send_status text NOT NULL DEFAULT 'stored' CHECK (send_status = 'stored'),
  delivery_status text NOT NULL DEFAULT 'not_sent' CHECK (delivery_status = 'not_sent'),
  provider_adopted boolean NOT NULL DEFAULT false CHECK (provider_adopted = false),
  hidden_at timestamptz,
  hidden_by uuid,
  hide_reason text NOT NULL DEFAULT '',
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (sender_id, idempotency_key)
);

CREATE INDEX messages_by_conversation ON messages.messages (conversation_id, created_at, id);

CREATE TABLE messages.reads (
  conversation_id uuid NOT NULL,
  user_id uuid NOT NULL,
  read_at timestamptz NOT NULL,
  PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE messages.reports (
  id uuid PRIMARY KEY,
  conversation_id uuid NOT NULL,
  reporter_id uuid NOT NULL,
  reason text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('abuse','block')),
  status text NOT NULL DEFAULT 'under_review' CHECK (status = 'under_review'),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE messages.canned (
  id uuid PRIMARY KEY,
  locale text NOT NULL CHECK (locale IN ('ar','en')),
  body text NOT NULL,
  active boolean NOT NULL DEFAULT true
);

INSERT INTO messages.canned (id, locale, body) VALUES
  ('55555555-5555-4555-8555-555555555501', 'ar', 'وصلنا رسالتك وسنراجعها داخل شمال.'),
  ('55555555-5555-4555-8555-555555555502', 'en', 'We have your message and will review it inside Shamal.');

CREATE OR REPLACE FUNCTION messages.guard_message() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'append_only' USING ERRCODE = '55000';
  END IF;
  IF NEW.body IS DISTINCT FROM OLD.body
     OR NEW.sender_id IS DISTINCT FROM OLD.sender_id
     OR NEW.kind IS DISTINCT FROM OLD.kind
     OR NEW.created_at IS DISTINCT FROM OLD.created_at
     OR NEW.conversation_id IS DISTINCT FROM OLD.conversation_id
     OR NEW.send_status IS DISTINCT FROM OLD.send_status
     OR NEW.delivery_status IS DISTINCT FROM OLD.delivery_status THEN
    RAISE EXCEPTION 'append_only' USING ERRCODE = '55000';
  END IF;
  IF OLD.hidden_at IS NOT NULL THEN
    RAISE EXCEPTION 'append_only' USING ERRCODE = '55000';
  END IF;
  RETURN NEW;
END $$;

CREATE TRIGGER messages_append_only
  BEFORE UPDATE OR DELETE ON messages.messages
  FOR EACH ROW EXECUTE FUNCTION messages.guard_message();

CREATE SEQUENCE disputes.case_serial START 1000;

CREATE TABLE disputes.cases (
  id uuid PRIMARY KEY,
  serial bigint NOT NULL UNIQUE DEFAULT nextval('disputes.case_serial'),
  company_id uuid NOT NULL,
  party_user_id uuid NOT NULL,
  opened_by uuid NOT NULL,
  subject_kind text NOT NULL CHECK (subject_kind IN ('booking','payment','service')),
  subject_id uuid NOT NULL,
  stay_id uuid,
  type text NOT NULL CHECK (type IN ('entry_failure','mismatch','cleanliness','unit_issue','service','compensation','refund')),
  description text NOT NULL,
  occurred_at timestamptz NOT NULL,
  requested_amount_minor bigint,
  currency text NOT NULL DEFAULT 'SAR' CHECK (currency = 'SAR'),
  status text NOT NULL CHECK (status IN (
    'open','classified','in_progress','awaiting_client','awaiting_landlord','escalated','proposed','resolved','closed','reopened'
  )),
  priority text NOT NULL CHECK (priority IN ('critical','normal')),
  sla_adopted boolean NOT NULL DEFAULT false CHECK (sla_adopted = false),
  contractual boolean NOT NULL DEFAULT false CHECK (contractual = false),
  response_due_at timestamptz NOT NULL,
  assignee_id uuid,
  proposal_kind text NOT NULL DEFAULT '',
  proposal_amount_minor bigint,
  proposal_reason text NOT NULL DEFAULT '',
  proposal_actions text NOT NULL DEFAULT '',
  proposed_by uuid,
  party_notice_at timestamptz,
  resolved_at timestamptz,
  policy_version text NOT NULL DEFAULT 'proposed-7d',
  policy_adopted boolean NOT NULL DEFAULT false CHECK (policy_adopted = false),
  financial_complete boolean NOT NULL DEFAULT false CHECK (financial_complete = false),
  money_moved boolean NOT NULL DEFAULT false CHECK (money_moved = false),
  execution_status text NOT NULL DEFAULT '',
  refund_id uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disputes.evidence (
  id uuid PRIMARY KEY,
  case_id uuid NOT NULL,
  sha256 text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
  storage_key text NOT NULL,
  uploaded_by uuid NOT NULL,
  uploaded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (case_id, sha256)
);

CREATE TABLE disputes.notes (
  id uuid PRIMARY KEY,
  case_id uuid NOT NULL,
  author_id uuid NOT NULL,
  visibility text NOT NULL CHECK (visibility IN ('party','internal')),
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disputes.events (
  id uuid PRIMARY KEY,
  case_id uuid NOT NULL,
  actor_id uuid NOT NULL,
  kind text NOT NULL,
  from_status text NOT NULL DEFAULT '',
  to_status text NOT NULL DEFAULT '',
  reason text NOT NULL DEFAULT '',
  notice_status text NOT NULL DEFAULT 'recorded' CHECK (notice_status = 'recorded'),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disputes.decisions (
  id uuid PRIMARY KEY,
  case_id uuid NOT NULL,
  kind text NOT NULL,
  amount_minor bigint,
  reason text NOT NULL,
  actions text NOT NULL,
  proposer_id uuid NOT NULL,
  approver_id uuid NOT NULL,
  refund_id uuid,
  execution_status text NOT NULL,
  money_moved boolean NOT NULL DEFAULT false CHECK (money_moved = false),
  financial_complete boolean NOT NULL DEFAULT false CHECK (financial_complete = false),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disputes.holds (
  case_id uuid PRIMARY KEY,
  amount_minor bigint NOT NULL CHECK (amount_minor > 0),
  refund_id uuid,
  other_funds_frozen boolean NOT NULL DEFAULT false CHECK (other_funds_frozen = false),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE disputes.reviews (
  id uuid PRIMARY KEY,
  case_id uuid NOT NULL,
  requester_id uuid NOT NULL,
  reason text NOT NULL,
  within_window boolean NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION disputes.reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'append_only' USING ERRCODE = '55000';
END $$;

CREATE TRIGGER evidence_append_only
  BEFORE UPDATE OR DELETE ON disputes.evidence
  FOR EACH ROW EXECUTE FUNCTION disputes.reject_change();

CREATE TRIGGER decisions_append_only
  BEFORE UPDATE OR DELETE ON disputes.decisions
  FOR EACH ROW EXECUTE FUNCTION disputes.reject_change();

CREATE TRIGGER dispute_events_append_only
  BEFORE UPDATE OR DELETE ON disputes.events
  FOR EACH ROW EXECUTE FUNCTION disputes.reject_change();

CREATE TRIGGER dispute_notes_append_only
  BEFORE UPDATE OR DELETE ON disputes.notes
  FOR EACH ROW EXECUTE FUNCTION disputes.reject_change();

CREATE TRIGGER holds_append_only
  BEFORE UPDATE OR DELETE ON disputes.holds
  FOR EACH ROW EXECUTE FUNCTION disputes.reject_change();

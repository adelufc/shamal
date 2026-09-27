CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE SCHEMA IF NOT EXISTS stays;

CREATE TABLE stays.policies (
  id uuid PRIMARY KEY,
  version int NOT NULL,
  transaction_type text NOT NULL CHECK (transaction_type IN ('daily','monthly')),
  contractual boolean NOT NULL DEFAULT false,
  summary_ar text NOT NULL,
  summary_en text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (transaction_type, version)
);

INSERT INTO stays.policies (id, version, transaction_type, contractual, summary_ar, summary_en) VALUES
(
  '33333333-3333-4333-8333-333333333301', 1, 'daily', false,
  'نسخة اختبار غير تعاقدية. لا يوجد جدول استرداد معتمد، ولا رسم إلغاء، ولا عمولة. قبول هذه النسخة يحفظها مع الحجز ولا ينشئ التزاماً مالياً.',
  'A non-contractual test copy. No refund schedule is adopted, no cancellation fee is set, and no commission is set. Accepting this copy stores it with the stay and does not create a financial obligation.'
),
(
  '33333333-3333-4333-8333-333333333302', 1, 'monthly', false,
  'نسخة اختبار غير تعاقدية للإيجار الشهري. لا يوجد جدول استرداد معتمد، ولا رسم إلغاء، ولا عمولة. قبول هذه النسخة يحفظها مع الحجز ولا ينشئ التزاماً مالياً.',
  'A non-contractual test copy for a monthly stay. No refund schedule is adopted, no cancellation fee is set, and no commission is set. Accepting this copy stores it with the stay and does not create a financial obligation.'
);

CREATE TABLE stays.bookings (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  listing_id uuid NOT NULL,
  unit_id uuid NOT NULL,
  profile_id uuid NOT NULL,
  actor_id uuid NOT NULL,
  transaction_type text NOT NULL CHECK (transaction_type IN ('daily','monthly')),
  arrival date NOT NULL,
  departure date NOT NULL,
  guests int NOT NULL CHECK (guests > 0),
  status text NOT NULL CHECK (status IN ('held','confirmed','expired','cancelled')),
  hold_expires_at timestamptz,
  currency text NOT NULL DEFAULT 'SAR',
  nights int NOT NULL DEFAULT 0,
  months int NOT NULL DEFAULT 0,
  base_minor bigint NOT NULL,
  discount_minor bigint NOT NULL DEFAULT 0,
  service_minor bigint NOT NULL DEFAULT 0,
  tax_minor bigint NOT NULL DEFAULT 0,
  deposit_minor bigint NOT NULL DEFAULT 0,
  stay_due_minor bigint NOT NULL,
  collect_minor bigint NOT NULL,
  commission_minor bigint,
  policy_id uuid NOT NULL,
  policy_version int NOT NULL,
  policy_summary_ar text NOT NULL,
  policy_summary_en text NOT NULL,
  address_ar text NOT NULL DEFAULT '',
  address_en text NOT NULL DEFAULT '',
  title_ar text NOT NULL DEFAULT '',
  title_en text NOT NULL DEFAULT '',
  accepted_policy_at timestamptz,
  quote_expires_at timestamptz NOT NULL,
  confirmed_at timestamptz,
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (departure > arrival),
  CHECK (collect_minor = stay_due_minor + deposit_minor),
  CHECK (tax_minor = 0),
  CHECK (service_minor = 0),
  CHECK (discount_minor = 0),
  CHECK (deposit_minor = 0),
  CHECK (commission_minor IS NULL),
  CHECK (stay_due_minor = base_minor)
);

CREATE TABLE stays.idempotency (
  actor_id uuid NOT NULL,
  idempotency_key text NOT NULL,
  request_hash text NOT NULL,
  booking_id uuid NOT NULL,
  PRIMARY KEY (actor_id, idempotency_key)
);

CREATE TABLE stays.occupancy (
  id uuid PRIMARY KEY,
  unit_id uuid NOT NULL,
  booking_id uuid,
  kind text NOT NULL CHECK (kind IN ('hold','confirmed','annual','closure','maintenance')),
  start_on date NOT NULL,
  end_on date NOT NULL,
  CHECK (end_on > start_on)
);

ALTER TABLE stays.occupancy
  ADD CONSTRAINT occupancy_no_overlap
  EXCLUDE USING gist (
    unit_id WITH =,
    daterange(start_on, end_on, '[)') WITH &&
  );

CREATE INDEX stays_bookings_actor ON stays.bookings (actor_id, created_at DESC);
CREATE INDEX stays_bookings_unit ON stays.bookings (unit_id, status);

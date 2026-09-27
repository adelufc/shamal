CREATE EXTENSION IF NOT EXISTS postgis;

CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS organization;
CREATE SCHEMA IF NOT EXISTS permissions;
CREATE SCHEMA IF NOT EXISTS geography;
CREATE SCHEMA IF NOT EXISTS catalog;
CREATE SCHEMA IF NOT EXISTS audit;
CREATE SCHEMA IF NOT EXISTS files;

CREATE TABLE identity.users (
  id uuid PRIMARY KEY,
  company_id uuid,
  status text NOT NULL CHECK (status IN ('pending_verification','active','locked','suspended','deletion_requested','logically_deleted')),
  display_name text NOT NULL,
  password_hash text,
  mfa_required boolean NOT NULL DEFAULT false,
  mfa_secret text,
  failed_attempts int NOT NULL DEFAULT 0,
  first_failed_at timestamptz,
  locked_until timestamptz,
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identity.contact_methods (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES identity.users(id),
  kind text NOT NULL CHECK (kind IN ('email','phone')),
  value text NOT NULL,
  verified_at timestamptz,
  is_primary boolean NOT NULL DEFAULT false,
  UNIQUE (kind, value)
);

CREATE TABLE identity.devices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES identity.users(id),
  label text NOT NULL,
  fingerprint_hash text NOT NULL,
  last_seen_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organization.nodes (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('company','branch','department','team')),
  parent_id uuid REFERENCES organization.nodes(id),
  name_ar text NOT NULL,
  name_en text NOT NULL,
  status text NOT NULL CHECK (status IN ('active','closed')),
  manager_user_id uuid,
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organization.memberships (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL,
  node_id uuid NOT NULL REFERENCES organization.nodes(id),
  company_id uuid NOT NULL,
  is_primary boolean NOT NULL DEFAULT false,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz,
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions.rules (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  grantee_type text NOT NULL CHECK (grantee_type IN ('user','node')),
  grantee_id uuid NOT NULL,
  domain text NOT NULL,
  action text NOT NULL,
  effect text NOT NULL CHECK (effect IN ('allow','deny','restrict')),
  layer text NOT NULL CHECK (layer IN ('explicit_deny','exception','direct','inherited')),
  scope_kind text NOT NULL,
  scope_node_id uuid,
  starts_at timestamptz,
  ends_at timestamptz,
  source_label text NOT NULL,
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid
);

CREATE TABLE permissions.inheritance_breaks (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  node_id uuid NOT NULL,
  domain text NOT NULL,
  action text NOT NULL,
  reason text NOT NULL,
  approved_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE geography.cities (
  id uuid PRIMARY KEY,
  country_code text NOT NULL,
  name_ar text NOT NULL,
  name_en text NOT NULL,
  timezone text NOT NULL,
  currency text NOT NULL,
  boundary geometry(MultiPolygon, 4326) NOT NULL,
  boundary_authority text NOT NULL,
  boundary_note_ar text NOT NULL,
  boundary_note_en text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE geography.neighborhoods (
  id uuid PRIMARY KEY,
  city_id uuid NOT NULL REFERENCES geography.cities(id),
  company_id uuid NOT NULL,
  branch_id uuid,
  status text NOT NULL CHECK (status IN ('draft','in_review','changes_requested','published','suspended','archived','merged')),
  version int NOT NULL DEFAULT 1,
  service_sale boolean NOT NULL DEFAULT false,
  service_annual boolean NOT NULL DEFAULT false,
  service_monthly boolean NOT NULL DEFAULT false,
  service_daily boolean NOT NULL DEFAULT false,
  coverage text NOT NULL CHECK (coverage IN ('directory_only','served')),
  show_in_directory boolean NOT NULL DEFAULT false,
  show_on_map boolean NOT NULL DEFAULT false,
  boundary geometry(MultiPolygon, 4326),
  boundary_source text,
  boundary_is_official boolean NOT NULL DEFAULT false,
  boundary_note_ar text NOT NULL DEFAULT '',
  boundary_note_en text NOT NULL DEFAULT '',
  display_point geometry(Point, 4326),
  merged_into_id uuid,
  validation_issues jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_by uuid NOT NULL,
  updated_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE geography.translations (
  neighborhood_id uuid NOT NULL REFERENCES geography.neighborhoods(id),
  city_id uuid NOT NULL,
  locale text NOT NULL CHECK (locale IN ('ar','en')),
  name text NOT NULL,
  normalized_name text NOT NULL,
  slug text NOT NULL,
  description text NOT NULL DEFAULT '',
  title text NOT NULL DEFAULT '',
  search_description text NOT NULL DEFAULT '',
  PRIMARY KEY (neighborhood_id, locale)
);

CREATE UNIQUE INDEX geography_slug_unique ON geography.translations (city_id, locale, slug);

CREATE TABLE geography.aliases (
  id uuid PRIMARY KEY,
  neighborhood_id uuid NOT NULL REFERENCES geography.neighborhoods(id),
  locale text NOT NULL,
  alias text NOT NULL,
  normalized text NOT NULL
);

CREATE TABLE geography.revisions (
  id uuid PRIMARY KEY,
  neighborhood_id uuid NOT NULL REFERENCES geography.neighborhoods(id),
  status text NOT NULL,
  snapshot jsonb NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE geography.slug_redirects (
  city_id uuid NOT NULL,
  locale text NOT NULL,
  from_slug text NOT NULL,
  neighborhood_id uuid NOT NULL,
  PRIMARY KEY (city_id, locale, from_slug)
);

CREATE TABLE geography.overlap_exceptions (
  id uuid PRIMARY KEY,
  neighborhood_id uuid NOT NULL,
  other_id uuid NOT NULL,
  reason text NOT NULL,
  approved_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE geography.sync_jobs (
  id uuid PRIMARY KEY,
  neighborhood_id uuid NOT NULL,
  status text NOT NULL CHECK (status IN ('recorded','running','succeeded','failed')),
  attempts int NOT NULL DEFAULT 0,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX geography_neighborhood_status ON geography.neighborhoods (city_id, status);
CREATE INDEX geography_boundary_gist ON geography.neighborhoods USING gist (boundary);

CREATE TABLE catalog.property_types (
  code text PRIMARY KEY,
  name_ar text NOT NULL,
  name_en text NOT NULL,
  requires_rooms boolean NOT NULL
);

CREATE TABLE catalog.properties (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  branch_id uuid,
  city_id uuid NOT NULL,
  neighborhood_id uuid NOT NULL,
  owner_user_id uuid,
  delegation_ref text NOT NULL DEFAULT '',
  precise_point geometry(Point, 4326),
  public_point geometry(Point, 4326),
  address_ar text NOT NULL DEFAULT '',
  address_en text NOT NULL DEFAULT '',
  type_code text NOT NULL REFERENCES catalog.property_types(code),
  area_sqm numeric(14,2),
  facade text NOT NULL DEFAULT '',
  street_width_m numeric(8,2),
  age_years int,
  readiness text NOT NULL DEFAULT '',
  asset_ref text NOT NULL DEFAULT '',
  image_rights text NOT NULL DEFAULT '',
  version int NOT NULL DEFAULT 1,
  created_by uuid NOT NULL,
  updated_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE catalog.units (
  id uuid PRIMARY KEY,
  property_id uuid NOT NULL REFERENCES catalog.properties(id),
  unit_number text NOT NULL,
  floor int,
  rooms int,
  baths int,
  capacity int,
  furnished boolean,
  bedding boolean,
  air_conditioning boolean,
  internet boolean,
  parking boolean,
  self_entry boolean,
  status text NOT NULL CHECK (status IN ('preparing','active','suspended','archived')),
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (property_id, unit_number)
);

CREATE TABLE catalog.listings (
  id uuid PRIMARY KEY,
  property_id uuid NOT NULL REFERENCES catalog.properties(id),
  unit_id uuid NOT NULL REFERENCES catalog.units(id),
  neighborhood_id uuid NOT NULL,
  company_id uuid NOT NULL,
  branch_id uuid,
  transaction_type text NOT NULL CHECK (transaction_type IN ('sale','annual','monthly','daily')),
  price_minor bigint,
  price_unit text NOT NULL,
  currency text NOT NULL DEFAULT 'SAR',
  valid_from date,
  valid_to date,
  license_ref text NOT NULL DEFAULT '',
  license_exception_ref text NOT NULL DEFAULT '',
  broker_user_id uuid,
  status text NOT NULL CHECK (status IN ('draft','in_review','changes_requested','approved','published','suspended','ended','archived')),
  version int NOT NULL DEFAULT 1,
  validation_issues jsonb NOT NULL DEFAULT '[]'::jsonb,
  pending_changes jsonb,
  created_by uuid NOT NULL,
  updated_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE catalog.listing_translations (
  listing_id uuid NOT NULL REFERENCES catalog.listings(id),
  locale text NOT NULL CHECK (locale IN ('ar','en')),
  title text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  PRIMARY KEY (listing_id, locale)
);

CREATE TABLE catalog.price_history (
  id uuid PRIMARY KEY,
  listing_id uuid NOT NULL REFERENCES catalog.listings(id),
  price_minor bigint NOT NULL,
  price_unit text NOT NULL,
  currency text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE catalog.location_waivers (
  id uuid PRIMARY KEY,
  listing_id uuid NOT NULL,
  reason text NOT NULL,
  approved_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit.events (
  id uuid PRIMARY KEY,
  actor_id uuid,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  company_id uuid,
  request_id text,
  result text NOT NULL,
  reason text NOT NULL DEFAULT '',
  approver_id uuid,
  before_data jsonb,
  after_data jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_entity ON audit.events (entity_type, entity_id, created_at);

CREATE TABLE files.objects (
  id uuid PRIMARY KEY,
  area text NOT NULL CHECK (area IN ('public','private')),
  storage_key text NOT NULL UNIQUE,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL,
  original_name text NOT NULL,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

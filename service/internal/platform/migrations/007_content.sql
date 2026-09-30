CREATE SCHEMA IF NOT EXISTS content;

CREATE TABLE content.pages (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('home','featured','offer','article','faq','policy','service')),
  slug text NOT NULL,
  neighborhood_id uuid,
  status text NOT NULL CHECK (status IN ('draft','in_review','published')),
  version int NOT NULL DEFAULT 1,
  created_by uuid NOT NULL,
  updated_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, kind, slug)
);

CREATE TABLE content.translations (
  page_id uuid NOT NULL REFERENCES content.pages(id),
  locale text NOT NULL CHECK (locale IN ('ar','en')),
  title text NOT NULL,
  body text NOT NULL,
  PRIMARY KEY (page_id, locale)
);

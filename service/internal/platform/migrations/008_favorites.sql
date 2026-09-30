CREATE TABLE catalog.favorites (
  user_id uuid NOT NULL REFERENCES identity.users(id),
  listing_id uuid NOT NULL REFERENCES catalog.listings(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, listing_id)
);

CREATE INDEX catalog_favorites_user ON catalog.favorites (user_id, created_at DESC);

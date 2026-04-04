-- +goose Up
-- Populate the search_vector column on every INSERT/UPDATE to listings.
-- The vector is built from title (weight A) and description (weight B).
-- This enables fast GIN-indexed fulltext search for the discovery feed.

CREATE OR REPLACE FUNCTION update_listing_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trig_listing_search_vector
BEFORE INSERT OR UPDATE OF title, description ON listings
FOR EACH ROW EXECUTE FUNCTION update_listing_search_vector();

-- Backfill existing rows.
UPDATE listings SET title = title WHERE search_vector IS NULL;

-- +goose Down
DROP TRIGGER IF EXISTS trig_listing_search_vector ON listings;
DROP FUNCTION IF EXISTS update_listing_search_vector();

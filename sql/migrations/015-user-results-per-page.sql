ALTER TABLE users ADD COLUMN results_per_page integer NOT NULL DEFAULT 50 CHECK (results_per_page > 0 AND results_per_page <= 500);

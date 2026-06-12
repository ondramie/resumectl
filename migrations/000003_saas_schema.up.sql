CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    api_token_hash TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS resume_templates (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    content_tex TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, name)
);

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES users(id);
ALTER TABLE match_runs ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES users(id);

ALTER TABLE match_runs ADD COLUMN IF NOT EXISTS resume_tex TEXT;
ALTER TABLE match_runs ADD COLUMN IF NOT EXISTS job_description TEXT;
ALTER TABLE match_runs ADD COLUMN IF NOT EXISTS report TEXT;

CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_match_runs_user_id ON match_runs(user_id);
CREATE INDEX IF NOT EXISTS idx_resume_templates_user_id ON resume_templates(user_id);

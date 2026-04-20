CREATE TABLE IF NOT EXISTS jobs (
    id SERIAL PRIMARY KEY,
    url TEXT UNIQUE,
    company TEXT,
    title TEXT,
    score INTEGER,
    status TEXT DEFAULT 'new',
    applied_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    role_type TEXT,
    industry TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS match_runs (
    id SERIAL PRIMARY KEY,
    job_id INTEGER REFERENCES jobs(id),
    score INTEGER,
    strong_matches TEXT,
    gaps TEXT,
    source_resume_hash TEXT,
    tailored_resume_hash TEXT,
    output_dir TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

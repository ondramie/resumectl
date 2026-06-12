DROP INDEX IF EXISTS idx_match_runs_user_id;
DROP INDEX IF EXISTS idx_jobs_user_id;

ALTER TABLE match_runs DROP COLUMN IF EXISTS report;
ALTER TABLE match_runs DROP COLUMN IF EXISTS job_description;
ALTER TABLE match_runs DROP COLUMN IF EXISTS resume_tex;

ALTER TABLE match_runs DROP COLUMN IF EXISTS user_id;
ALTER TABLE jobs DROP COLUMN IF EXISTS user_id;

DROP TABLE IF EXISTS users;

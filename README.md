# resume-tailor

CLI tool that matches your resume against job postings and tailors it for better ATS scores.

## Features

- **Match & Score** - Analyzes resume against job descriptions, scores 0-100
- **Auto-Tailor** - Reorders bullet points and highlights relevant skills
- **Cover Letters** - Optional cover letter generation
- **Local Database** - Saves jobs to SQLite (`~/.resume-tailor/data.db`)
- **Multiple ATS** - Supports Greenhouse, Lever, Ashby, Rippling, generic pages

## Prerequisites

- Go 1.20+
- [Anthropic API key](https://console.anthropic.com/)
- [tectonic](https://tectonic-typesetting.github.io/) (for PDF compilation)
- [direnv](https://direnv.net/) (optional)

## Setup

1. **Build:**
   ```bash
   go build -o resume-tailor .
   ```

2. **Set API key:**
   ```bash
   # Option A: direnv (recommended)
   echo 'export ANTHROPIC_API_KEY=your-key' > .env
   echo 'dotenv' > .envrc
   direnv allow

   # Option B: export directly
   export ANTHROPIC_API_KEY=your-key
   ```

3. **Create resume template** (LaTeX):
   ```bash
   # Edit resume.template.tex with your info
   ```

## Usage

### Match a job posting
```bash
./resume-tailor match "https://jobs.lever.co/company/job-id"
./resume-tailor match "https://boards.greenhouse.io/company/jobs/123"

# From file (for blocked sites)
./resume-tailor match -f jobs/description.txt "CompanyName"

# With cover letter
./resume-tailor match --cover-letter "https://..."
```

### List saved jobs
```bash
./resume-tailor list
./resume-tailor list --min-score 75
./resume-tailor list --status new
```

### Compile PDF
```bash
./resume-tailor pdf results/company/job-id
```

## Output

Results saved to `results/{company}/{job-id}/`:
- `resume.tex` - Tailored resume
- `job.txt` - Job description
- `report.txt` - Match analysis
- `cover-letter.txt` - Cover letter (if requested)

## Supported Job Boards

| Board | Method |
|-------|--------|
| Greenhouse | API |
| Lever | API |
| Ashby | API |
| Rippling | Scrape |
| Generic | Scrape + JSON-LD |

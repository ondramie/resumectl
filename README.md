# resumectl

Self-custodial job hunting. You own your data.

> Like a crypto wallet for your job search — no SaaS platform owns your history.

## Features

- **Match & Score** - Analyzes resume against job descriptions, scores 0-100
- **Auto-Tailor** - Reorders bullet points and highlights relevant skills
- **Cover Letters** - Optional cover letter generation
- **Local Database** - All data in SQLite (`~/.resumectl/data.db`)
- **Multiple ATS** - Supports Greenhouse, Lever, Ashby, Rippling, generic pages

## Prerequisites

- Go 1.20+
- [Anthropic API key](https://console.anthropic.com/)
- [tectonic](https://tectonic-typesetting.github.io/) (for PDF compilation)
- [direnv](https://direnv.net/) (optional)

## Setup

```bash
# Build
go build -o resumectl .

# Set API key (one time)
echo 'export ANTHROPIC_API_KEY=your-key' > .env
echo 'dotenv' > .envrc
direnv allow

# Or export directly
export ANTHROPIC_API_KEY=your-key
```

## Usage

### Match a job posting
```bash
resumectl match "https://jobs.lever.co/company/job-id"
resumectl match "https://boards.greenhouse.io/company/jobs/123"

# From file (for sites without API)
resumectl match -f jobs/description.txt "CompanyName"

# With cover letter
resumectl match --cover-letter "https://..."
```

### List saved jobs
```bash
resumectl list
resumectl list --min-score 75
resumectl list --status new
```

### Compile PDF
```bash
resumectl pdf results/company/job-id
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

## Privacy (Self-Custodial)

- All data stored locally (`~/.resumectl/`)
- Claude API is stateless (no data retention)
- Delete everything: `rm -rf ~/.resumectl`
- Export: just copy the folder

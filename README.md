# resumectl

Self-custodial job hunting CLI. Match your resume against job postings, generate tailored PDFs, and track your pipeline — from the terminal or your iPhone.

## Features

- **Match & Score** — Analyzes resume against job descriptions, scores 0-100
- **Auto-Tailor** — Reorders bullets, highlights relevant skills, compiles a PDF
- **Template Selection** — Picks the best resume template for each job automatically
- **Pipeline Dashboard** — Funnel, rejection turnaround, active applications by industry/role
- **iOS Share Extension** — Share a job URL from Safari, get a tailored PDF on your phone
- **Shared Database** — CLI and iOS app sync through Neon Postgres

## Supported Job Boards

| Board | Method |
|-------|--------|
| Greenhouse | API |
| Lever | API |
| Ashby | API |
| Workable | API |
| Rippling | Scrape |
| Generic / PDF | Scrape + JSON-LD |

## Prerequisites

- Go 1.21+
- [Anthropic API key](https://console.anthropic.com/)
- [tectonic](https://tectonic-typesetting.github.io/) (for PDF compilation)
- [Neon](https://neon.tech) Postgres database

## Setup

```bash
go build -o resumectl .
```

Set environment variables (`.env` or `.envrc`):

```bash
export ANTHROPIC_API_KEY=your-key
export NEON_DB_URL=postgresql://...
export RESUMECTL_API_TOKEN=your-token   # for the HTTP server
```

## CLI Usage

```bash
# Match a job posting
resumectl match "https://jobs.lever.co/company/job-id"
resumectl match "https://boards.greenhouse.io/company/jobs/123"

# List saved jobs
resumectl list
resumectl list --min-score 80
resumectl list --status applied

# Update a job's status
resumectl status confluent applied
resumectl status figma rejected
resumectl status 42 interview

# View pipeline dashboard
resumectl pipeline
resumectl pipeline --by industry
resumectl pipeline --by role

# Compile PDF from existing tailored resume
resumectl pdf results/company/job-id

# Scan job boards
resumectl scan -q "data engineer,data platform" --board all --location remote

# Run HTTP server (for iOS app)
resumectl serve --port 8080
```

## HTTP Server

The `serve` command exposes a REST API used by the iOS app:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/match` | POST | Match + tailor resume, returns score and PDF URL |
| `/pipeline` | GET | Pipeline summary |
| `/pdf/:path` | GET | Download generated PDF |
| `/health` | GET | Health check |

All endpoints require `Authorization: Bearer <RESUMECTL_API_TOKEN>`.

## iOS App

Located in `ios-resumectl/`. Built with xcodegen.

```bash
cd ios-resumectl
make install   # builds and pushes to connected iPhone
```

Share any job URL from Safari → the extension matches your resume, tailors it, and saves the PDF directly to your Files app.

## Output

Results saved to `results/{company}/{job-id}/`:
- `resume.tex` — Tailored LaTeX resume
- `resume.pdf` — Compiled PDF
- `job.txt` — Job description
- `report.txt` — Match analysis

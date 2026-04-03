package main

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func stripHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		re := regexp.MustCompile(`<[^>]*>`)
		return re.ReplaceAllString(html, "")
	}
	text := doc.Text()
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractReqIDFromURL(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		for _, key := range []string{"pid", "req", "id", "jid", "gh_jid"} {
			if v := u.Query().Get(key); v != "" {
				return v
			}
		}
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/jobs/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`),
		regexp.MustCompile(`/jobs/(\d+)`),
		regexp.MustCompile(`/job/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`),
		regexp.MustCompile(`/job/(\d+)`),
		regexp.MustCompile(`/positions/(\d+)`),
	}

	for _, p := range patterns {
		if matches := p.FindStringSubmatch(rawURL); matches != nil {
			return matches[1]
		}
	}

	return ""
}

func extractCompanyFromURL(rawURL string) string {
	patterns := []struct {
		re    *regexp.Regexp
		group int
	}{
		{regexp.MustCompile(`apply\.workable\.com/([^/]+)/`), 1},
		{regexp.MustCompile(`/companies/([^/]+)/jobs/`), 1},
		{regexp.MustCompile(`ats\.rippling\.com/([^/]+)/jobs/`), 1},
		{regexp.MustCompile(`boards\.greenhouse\.io/([^/]+)/`), 1},
		{regexp.MustCompile(`jobs\.lever\.co/([^/]+)/`), 1},
		{regexp.MustCompile(`://([^.]+)\.workday\.com`), 1},
		{regexp.MustCompile(`careers\.([^.]+)\.com`), 1},
		{regexp.MustCompile(`://([^.]+)\.bamboohr\.com`), 1},
		{regexp.MustCompile(`jobs\.ashbyhq\.com/([^/]+)`), 1},
	}

	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(rawURL); matches != nil {
			return matches[p.group]
		}
	}

	re := regexp.MustCompile(`https?://(?:www\.)?([^/]+)`)
	if matches := re.FindStringSubmatch(rawURL); matches != nil {
		parts := strings.Split(matches[1], ".")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return "unknown"
}

func filterFalseGaps(gaps []string, resume string) []string {
	resumeLower := strings.ToLower(resume)
	techTerms := regexp.MustCompile(
		`(?i)\b(` +
			`[A-Z][a-z]*(?:\.js|\.net|\.io)|` +
			`[A-Z][a-zA-Z]*DB|` +
			`Go|Rust|Ruby|Rails|Python|Java|Scala|Kotlin|Swift|Elixir|` +
			`TypeScript|JavaScript|C\+\+|C#|PHP|Perl|R\b|SQL|NoSQL|` +
			`Kafka|Flink|Spark|Hadoop|Hive|Presto|Airflow|Prefect|dbt|` +
			`Snowflake|Redshift|BigQuery|ClickHouse|Postgres|MySQL|MongoDB|DynamoDB|Cassandra|Redis|` +
			`Docker|Kubernetes|Terraform|Ansible|Jenkins|` +
			`AWS|GCP|Azure|Lambda|ECS|Fargate|S3|` +
			`React|Angular|Vue|Next\.js|Svelte|Django|Flask|FastAPI|Spring|Rails|Laravel|` +
			`GraphQL|REST|gRPC|Protobuf|` +
			`Elasticsearch|Kibana|Grafana|Prometheus|Datadog|` +
			`Git|CI/CD|ArgoCD|CircleCI|` +
			`Meltano|Looker|Tableau|dbt|Iceberg|DeltaLake|Delta\s?Lake|Debezium|` +
			`Kinesis|RabbitMQ|Pulsar|NATS|SQS|` +
			`TensorFlow|PyTorch|scikit-learn|MLflow` +
			`)\b`)
	var filtered []string
	for _, gap := range gaps {
		matches := techTerms.FindAllString(gap, -1)
		found := false
		for _, m := range matches {
			if strings.Contains(resumeLower, strings.ToLower(m)) {
				found = true
				break
			}
		}
		if !found {
			filtered = append(filtered, gap)
		}
	}
	return filtered
}

func postProcessLatex(latex string) string {
	tabularRe := regexp.MustCompile(`(?m)^.*\\begin\{tabular\}.*$`)
	latex = tabularRe.ReplaceAllString(latex,
		`\begin{tabular}{ @{} >{\bfseries}l @{\hspace{3ex}} >{\raggedright\arraybackslash}p{0.68\textwidth} }`)

	if !strings.Contains(latex, `\hyphenpenalty`) {
		latex = strings.Replace(latex,
			`\usepackage[left=0.75in,top=0.6in,right=0.75in,bottom=0.6in]{geometry}`,
			`\usepackage[left=0.75in,top=0.6in,right=0.75in,bottom=0.6in]{geometry}`+"\n"+
				`\tolerance=1`+"\n"+
				`\emergencystretch=\maxdimen`+"\n"+
				`\hyphenpenalty=10000`+"\n"+
				`\hbadness=10000`,
			1)
	}

	return latex
}

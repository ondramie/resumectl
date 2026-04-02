package main

import (
	"testing"
)

func TestExtractReqIDFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"greenhouse query param", "https://www.instacart.careers/job?gh_jid=7171161&gh_src=26e143c51", "7171161"},
		{"generic query pid", "https://example.com/careers?pid=12345", "12345"},
		{"numeric jobs path", "https://jobs.solana.com/companies/very-ai/jobs/71729730-full-stack-engineer#content", "71729730"},
		{"uuid jobs path", "https://example.com/jobs/a1b2c3d4-e5f6-7890-abcd-ef1234567890", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"uuid job path", "https://example.com/job/a1b2c3d4-e5f6-7890-abcd-ef1234567890", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"numeric job path", "https://example.com/job/12345", "12345"},
		{"positions path", "https://www.coinbase.com/careers/positions/7573222", "7573222"},
		{"no match", "https://example.com/careers/apply", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReqIDFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractReqIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractCompanyFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"workable", "https://apply.workable.com/anza-xyz/j/C3154D9B03/", "anza-xyz"},
		{"companies path", "https://jobs.solana.com/companies/very-ai/jobs/71729730", "very-ai"},
		{"rippling", "https://ats.rippling.com/acme/jobs/123", "acme"},
		{"greenhouse", "https://boards.greenhouse.io/acme/jobs/123", "acme"},
		{"lever", "https://jobs.lever.co/acme/abc-123", "acme"},
		{"workday", "https://acme.workday.com/en-US/job/123", "acme"},
		{"careers subdomain", "https://careers.acme.com/jobs/123", "acme"},
		{"bamboohr", "https://acme.bamboohr.com/careers/123", "acme"},
		{"ashby", "https://jobs.ashbyhq.com/acme/abc-123", "acme"},
		{"www stripped", "https://www.instacart.careers/job?gh_jid=7171161", "instacart"},
		{"plain domain", "https://example.com/careers/apply", "example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCompanyFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractCompanyFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestFilterFalseGaps(t *testing.T) {
	resume := `Built Cashback rewards service (Ruby/Rails) aggregating data.
Built real-time data pipelines (Kafka, Flink) ingesting blockchain state.
Experience with Rust and Python.`

	tests := []struct {
		name     string
		gaps     []string
		wantLen  int
		wantGaps []string
	}{
		{
			"filters rails gap",
			[]string{"Ruby/Rails web framework experience", "DeltaLake experience"},
			1,
			[]string{"DeltaLake experience"},
		},
		{
			"filters flink gap",
			[]string{"Flink streaming experience", "Iceberg experience"},
			1,
			[]string{"Iceberg experience"},
		},
		{
			"filters rust gap",
			[]string{"Rust experience", "Go experience"},
			1,
			[]string{"Go experience"},
		},
		{
			"keeps genuine gaps",
			[]string{"Kubernetes experience", "DeltaLake experience"},
			2,
			[]string{"Kubernetes experience", "DeltaLake experience"},
		},
		{
			"empty gaps",
			[]string{},
			0,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterFalseGaps(tt.gaps, resume)
			if len(got) != tt.wantLen {
				t.Errorf("filterFalseGaps() returned %d gaps, want %d: %v", len(got), tt.wantLen, got)
			}
			for i, want := range tt.wantGaps {
				if i < len(got) && got[i] != want {
					t.Errorf("gap[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

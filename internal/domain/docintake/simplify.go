package docintake

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Status string
type SourceKind string

const (
	StatusAnalyzed  Status = "analyzed"
	StatusFallback  Status = "fallback_structured"
)

const (
	KindCSV   SourceKind = "csv"
	KindJSON  SourceKind = "json"
	KindText  SourceKind = "txt"
	KindOPERA SourceKind = "opera_report"
)

const (
	MaxFileBytes   = 1 << 20 // 1 MiB
	MaxExcerptRune = 2400
	MaxFileName    = 140
)

type KeyMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type SimplifiedSummary struct {
	Headline        string      `json:"headline"`
	KeyMetrics      []KeyMetric `json:"keyMetrics"`
	Takeaways       []string    `json:"takeaways"`
	Risks           []string    `json:"risks"`
	Actions         []string    `json:"actions"`
	RowCount        int         `json:"rowCount"`
	ColumnCount     int         `json:"columnCount"`
	MissingCells    int         `json:"missingCells"`
	DataCoveragePct int         `json:"dataCoveragePct"`
}

type Document struct {
	ID              id.ID
	WorkspaceID     id.ID
	UploadedBy      id.ID
	BusinessDate    string
	FileName        string
	SourceKind      SourceKind
	ContentSHA256   string
	RawExcerpt      string
	Status          Status
	AIUsed          bool
	DataCoveragePct int
	Summary         SimplifiedSummary
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func DetectSourceKind(fileName, explicit, content string) SourceKind {
	s := strings.ToLower(strings.TrimSpace(explicit))
	switch SourceKind(s) {
	case KindCSV, KindJSON, KindText, KindOPERA:
		return SourceKind(s)
	}
	lower := strings.ToLower(fileName)
	switch {
	case strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".tsv"):
		return KindCSV
	case strings.HasSuffix(lower, ".json"):
		return KindJSON
	case strings.Contains(lower, "opera") || strings.Contains(lower, "pms"):
		return KindOPERA
	}
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return KindJSON
	}
	if strings.Contains(trimmed, "\n") && strings.Contains(trimmed, ",") {
		return KindCSV
	}
	return KindText
}

func NormalizeDocument(workspaceID, uploader id.ID, date, fileName, kind, content string, now time.Time) (Document, error) {
	if workspaceID == id.Zero() || uploader == id.Zero() {
		return Document{}, errx.Invalid("invalid_document_refs", "workspace and uploader are required")
	}
	if len(date) != 10 {
		return Document{}, errx.Invalid("invalid_business_date", "businessDate must be YYYY-MM-DD")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Document{}, errx.Invalid("invalid_business_date", "businessDate must be YYYY-MM-DD")
	}
	if len(content) > MaxFileBytes {
		return Document{}, errx.Invalid("file_too_large", "file content exceeds 1 MB limit")
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return Document{}, errx.Invalid("empty_document", "document content is required")
	}
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = "report.txt"
	}
	if r := []rune(name); len(r) > MaxFileName {
		name = string(r[:MaxFileName])
	}
	src := DetectSourceKind(name, kind, trimmed)
	sum := sha256.Sum256([]byte(trimmed))
	excerpt := trimmed
	if r := []rune(excerpt); len(r) > MaxExcerptRune {
		excerpt = string(r[:MaxExcerptRune])
	}
	summary := SimplifyDeterministic(name, src, trimmed)
	return Document{
		ID:              id.New(),
		WorkspaceID:     workspaceID,
		UploadedBy:      uploader,
		BusinessDate:    date,
		FileName:        name,
		SourceKind:      src,
		ContentSHA256:   hex.EncodeToString(sum[:]),
		RawExcerpt:      excerpt,
		Status:          StatusFallback,
		AIUsed:          false,
		DataCoveragePct: summary.DataCoveragePct,
		Summary:         summary,
		CreatedAt:       now.UTC(),
		UpdatedAt:       now.UTC(),
	}, nil
}

// SimplifyDeterministic inspects CSV, JSON or text reports without external AI.
func SimplifyDeterministic(fileName string, kind SourceKind, content string) SimplifiedSummary {
	switch kind {
	case KindCSV:
		if out, ok := simplifyCSV(fileName, content); ok {
			return out
		}
	case KindJSON:
		if out, ok := simplifyJSON(fileName, content); ok {
			return out
		}
	}
	return simplifyText(fileName, content)
}

func simplifyCSV(fileName, content string) (SimplifiedSummary, bool) {
	r := csv.NewReader(bytes.NewBufferString(content))
	if strings.HasSuffix(strings.ToLower(fileName), ".tsv") || (!strings.Contains(content, ",") && strings.Contains(content, "\t")) {
		r.Comma = '\t'
	}
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil && err != io.EOF {
		return SimplifiedSummary{}, false
	}
	if len(rows) < 2 {
		return SimplifiedSummary{}, false
	}
	headers := rows[0]
	dataRows := rows[1:]
	colCount := len(headers)
	totalCells := len(dataRows) * max(colCount, 1)
	nonEmpty, missing := 0, 0
	type numAgg struct {
		count int
		sum   float64
	}
	aggs := make([]numAgg, colCount)
	for _, row := range dataRows {
		for c := 0; c < colCount; c++ {
			val := ""
			if c < len(row) {
				val = strings.TrimSpace(row[c])
			}
			if val == "" || strings.EqualFold(val, "null") || strings.EqualFold(val, "n/a") || val == "-" {
				missing++
				continue
			}
			nonEmpty++
			cleaned := strings.ReplaceAll(strings.TrimPrefix(val, "$"), ",", "")
			if num, err := strconv.ParseFloat(cleaned, 64); err == nil && !math.IsNaN(num) && !math.IsInf(num, 0) {
				aggs[c].count++
				aggs[c].sum += num
			}
		}
	}
	cov := 0
	if totalCells > 0 {
		cov = (nonEmpty * 100) / totalCells
	}
	metrics := []KeyMetric{
		{Label: "Rows", Value: strconv.Itoa(len(dataRows))},
		{Label: "Columns", Value: strconv.Itoa(colCount)},
		{Label: "Completeness", Value: fmt.Sprintf("%d%%", cov)},
	}
	for c, h := range headers {
		if aggs[c].count > 0 && len(metrics) < 8 {
			label := strings.TrimSpace(h)
			if label == "" {
				label = fmt.Sprintf("Column %d", c+1)
			}
			metrics = append(metrics, KeyMetric{
				Label: label + " total",
				Value: formatFloat(aggs[c].sum),
			})
		}
	}
	takeaways := []string{
		fmt.Sprintf("Parsed %d rows across %d columns (%d%% filled).", len(dataRows), colCount, cov),
	}
	var risks []string
	var actions []string
	if missing > 0 {
		risks = append(risks, fmt.Sprintf("%d missing cell(s) detected across submitted rows.", missing))
		actions = append(actions, "Review missing cells with the responsible department before posting.")
	} else {
		actions = append(actions, "Verify totals against accounting/PMS source before approval.")
	}
	return SimplifiedSummary{
		Headline:        fmt.Sprintf("%s — %d rows, %d%% data coverage", fileName, len(dataRows), cov),
		KeyMetrics:      metrics,
		Takeaways:       takeaways,
		Risks:           risks,
		Actions:         actions,
		RowCount:        len(dataRows),
		ColumnCount:     colCount,
		MissingCells:    missing,
		DataCoveragePct: cov,
	}, true
}

func simplifyJSON(fileName, content string) (SimplifiedSummary, bool) {
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return SimplifiedSummary{}, false
	}
	var totalFields, nonEmpty, missing int
	metrics := make([]KeyMetric, 0, 6)
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				next := k
				if prefix != "" {
					next = prefix + "." + k
				}
				walk(next, val)
			}
		case []any:
			totalFields++
			if len(x) == 0 {
				missing++
			} else {
				nonEmpty++
				if len(metrics) < 6 {
					metrics = append(metrics, KeyMetric{Label: prefix + " items", Value: strconv.Itoa(len(x))})
				}
				for i, item := range x {
					if i >= 20 {
						break
					}
					walk(prefix, item)
				}
			}
		case nil:
			totalFields++
			missing++
		case string:
			totalFields++
			if strings.TrimSpace(x) == "" {
				missing++
			} else {
				nonEmpty++
			}
		case float64:
			totalFields++
			nonEmpty++
			if len(metrics) < 6 && prefix != "" {
				metrics = append(metrics, KeyMetric{Label: prefix, Value: formatFloat(x)})
			}
		default:
			totalFields++
			nonEmpty++
		}
	}
	walk("", parsed)
	cov := 100
	if totalFields > 0 {
		cov = (nonEmpty * 100) / totalFields
	}
	var risks []string
	if missing > 0 {
		risks = append(risks, fmt.Sprintf("%d empty or null JSON fields found.", missing))
	}
	return SimplifiedSummary{
		Headline:        fmt.Sprintf("%s — structured JSON (%d%% filled)", fileName, cov),
		KeyMetrics:      metrics,
		Takeaways:       []string{fmt.Sprintf("Extracted %d non-empty fields out of %d inspected.", nonEmpty, totalFields)},
		Risks:           risks,
		Actions:         []string{"Confirm extracted totals and assign follow-ups if any field is missing."},
		RowCount:        1,
		ColumnCount:     len(metrics),
		MissingCells:    missing,
		DataCoveragePct: cov,
	}, true
}

var numRe = regexp.MustCompile(`(?i)(USD|IQD|EUR|TRY|\$|%)?\s*\d[\d,]*(?:\.\d+)?%?`)
var riskWords = []string{"overdue", "variance", "missing", "delay", "late", "cancelled", "canceled", "out of order", "error", "shortage", "eksik", "gecik", "arıza", "iptal", "نقص", "تأخير", "عطل"}

func simplifyText(fileName, content string) SimplifiedSummary {
	lines := strings.Split(content, "\n")
	nonEmpty := 0
	var takeaways []string
	var risks []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		nonEmpty++
		if len(takeaways) < 4 && utf8.RuneCountInString(line) >= 6 {
			if r := []rune(line); len(r) > 140 {
				line = string(r[:140]) + "…"
			}
			takeaways = append(takeaways, line)
		}
		lower := strings.ToLower(raw)
		for _, w := range riskWords {
			if strings.Contains(lower, w) && len(risks) < 4 {
				risks = append(risks, line)
				break
			}
		}
	}
	nums := numRe.FindAllString(content, 6)
	metrics := []KeyMetric{{Label: "Lines", Value: strconv.Itoa(nonEmpty)}}
	for i, n := range nums {
		metrics = append(metrics, KeyMetric{Label: fmt.Sprintf("Figure %d", i+1), Value: strings.TrimSpace(n)})
	}
	cov := 100
	if len(lines) > 0 {
		cov = min(100, (nonEmpty*100)/len(lines))
	}
	return SimplifiedSummary{
		Headline:        fmt.Sprintf("%s — %d lines summarized", fileName, nonEmpty),
		KeyMetrics:      metrics,
		Takeaways:       takeaways,
		Risks:           risks,
		Actions:         []string{"Review highlighted risks and numbers before confirming action items."},
		RowCount:        nonEmpty,
		ColumnCount:     1,
		DataCoveragePct: cov,
	}
}

func formatFloat(v float64) string {
	if math.Abs(v-math.Round(v)) < 1e-6 {
		return strconv.FormatInt(int64(math.Round(v)), 10)
	}
	return fmt.Sprintf("%.2f", v)
}

package rag

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

const maxCSVRows = 1000

type Page struct {
	Text    string
	PageNum int
}

// Extractor extracts textual pages from raw bytes.
// It performs MIME sniffing and rejects dangerous/binary types.
type Extractor struct{}

func NewExtractor() *Extractor { return &Extractor{} }

// ExtractText returns the concatenated text and the detected MIME type.
// fileName is used only as a fallback hint for MIME type.
func (e *Extractor) ExtractText(_ context.Context, declaredMime string, data []byte, fileName string) (string, string, error) {
	m := DetectMimeType(data, fileName, declaredMime)
	// Reject dangerous binary blobs.
	if strings.Contains(m, "application/x-msdownload") || strings.Contains(strings.ToLower(fileName), ".exe") {
		return "", m, fmt.Errorf("unsupported file type")
	}
	pages, err := ExtractByMime(m, data)
	if err != nil {
		return "", m, err
	}
	var sb strings.Builder
	for i, p := range pages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(strings.TrimSpace(p.Text))
	}
	return strings.TrimSpace(sb.String()), m, nil
}

func DetectMimeType(data []byte, fileName, declared string) string {
	sniff := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	if strings.Contains(sniff, "application/octet-stream") || sniff == "" {
		if ext := strings.ToLower(filepath.Ext(fileName)); ext != "" {
			switch ext {
			case ".md":
				return "text/markdown"
			case ".txt":
				return "text/plain"
			case ".csv":
				return "text/csv"
			case ".json":
				return "application/json"
			case ".html", ".htm":
				return "text/html"
			case ".pdf":
				return "application/pdf"
			}
		}
	}
	if strings.HasPrefix(sniff, "text/plain") && declared != "" {
		dm, _, _ := mime.ParseMediaType(declared)
		if dm == "text/markdown" || dm == "text/csv" {
			return dm
		}
	}
	if i := strings.IndexByte(sniff, ';'); i > 0 {
		return sniff[:i]
	}
	return sniff
}

func ExtractByMime(mimeType string, data []byte) ([]Page, error) {
	switch mimeType {
	case "text/plain", "text/markdown":
		return []Page{{Text: string(data), PageNum: 1}}, nil
	case "application/json":
		return extractJSON(data)
	case "text/csv":
		return extractCSV(data)
	case "text/html":
		return extractHTML(data)
	case "application/pdf":
		// PDF extraction is intentionally disabled in v1 to avoid introducing
		// external runtime dependencies (poppler) or large pure-Go parsers.
		return nil, fmt.Errorf("pdf extraction is not enabled in this build")
	default:
		return nil, fmt.Errorf("unsupported mime type: %s", mimeType)
	}
}

func extractJSON(data []byte) ([]Page, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format json: %w", err)
	}
	return []Page{{Text: string(out), PageNum: 1}}, nil
}

func extractCSV(data []byte) ([]Page, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	rows := make([][]string, 0, 128)
	for len(rows) < maxCSVRows {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse csv: %w", err)
		}
		rows = append(rows, rec)
	}
	if len(rows) == 0 {
		return []Page{{Text: "", PageNum: 1}}, nil
	}
	var sb strings.Builder
	for i, r := range rows {
		sb.WriteString("| ")
		sb.WriteString(strings.Join(r, " | "))
		sb.WriteString(" |\n")
		if i == 0 {
			sb.WriteString("|")
			for range r {
				sb.WriteString(" --- |")
			}
			sb.WriteByte('\n')
		}
	}
	return []Page{{Text: sb.String(), PageNum: 1}}, nil
}

func extractHTML(data []byte) ([]Page, error) {
	s := string(data)
	replacer := strings.NewReplacer(
		"<script", "\n<script",
		"</script>", "</script>\n",
		"<style", "\n<style",
		"</style>", "</style>\n",
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
		"</li>", "\n",
	)
	s = replacer.Replace(s)
	for {
		start := strings.IndexByte(s, '<')
		if start < 0 {
			break
		}
		end := strings.IndexByte(s[start:], '>')
		if end < 0 {
			break
		}
		end += start
		tag := strings.ToLower(s[start : end+1])
		if strings.HasPrefix(tag, "<script") {
			closeIdx := strings.Index(strings.ToLower(s[end+1:]), "</script>")
			if closeIdx >= 0 {
				s = s[:start] + s[end+1+closeIdx+9:]
				continue
			}
		}
		if strings.HasPrefix(tag, "<style") {
			closeIdx := strings.Index(strings.ToLower(s[end+1:]), "</style>")
			if closeIdx >= 0 {
				s = s[:start] + s[end+1+closeIdx+8:]
				continue
			}
		}
		s = s[:start] + " " + s[end+1:]
	}
	s = strings.Join(strings.Fields(s), " ")
	return []Page{{Text: s, PageNum: 1}}, nil
}

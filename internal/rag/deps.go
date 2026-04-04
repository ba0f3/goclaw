package rag

import (
	"os/exec"
	"strings"
)

// DepsReport summarizes which attachment types can be extracted for RAG indexing.
type DepsReport struct {
	Supported   []string `json:"supported"`
	Unsupported []string `json:"unsupported"`
	Warnings    []string `json:"warnings"`
}

func runCmd(name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func hasPythonImport(module string) bool {
	return runCmd("python3", "-c", "import "+module)
}

func hasPandoc() bool {
	_, err := exec.LookPath("pandoc")
	return err == nil
}

// CheckDeps probes optional extractors and returns supported vs legacy-only types.
func CheckDeps() DepsReport {
	var rep DepsReport
	rep.Supported = []string{".md", ".txt", ".csv"}

	pypdfOK := hasPythonImport("pypdf")
	plumberOK := hasPythonImport("pdfplumber")
	switch {
	case pypdfOK || plumberOK:
		rep.Supported = append(rep.Supported, ".pdf")
	default:
		rep.Unsupported = append(rep.Unsupported, ".pdf")
		rep.Warnings = append(rep.Warnings, "pypdf/pdfplumber not found: .pdf will use legacy flow")
	}

	if hasPandoc() {
		rep.Supported = append(rep.Supported, ".docx", ".odt", ".epub")
	} else {
		rep.Unsupported = append(rep.Unsupported, ".docx", ".odt", ".epub")
		rep.Warnings = append(rep.Warnings, "pandoc not found: .docx/.odt/.epub will use legacy flow")
	}

	if hasPythonImport("openpyxl") {
		rep.Supported = append(rep.Supported, ".xlsx")
	} else {
		rep.Unsupported = append(rep.Unsupported, ".xlsx")
		rep.Warnings = append(rep.Warnings, "openpyxl not found: .xlsx will use legacy flow")
	}

	if hasPythonImport("pptx") {
		rep.Supported = append(rep.Supported, ".pptx")
	} else {
		rep.Unsupported = append(rep.Unsupported, ".pptx")
		rep.Warnings = append(rep.Warnings, "python-pptx not found: .pptx will use legacy flow")
	}

	return rep
}

// DepsReportJSON is the API shape for agent update responses.
func (d DepsReport) MarshalForAPI() map[string]any {
	return map[string]any{
		"supported":   d.Supported,
		"unsupported": d.Unsupported,
		"warnings":    d.Warnings,
	}
}

// NormalizeExt returns lowercase extension with a leading dot.
func NormalizeExt(ext string) string {
	e := strings.TrimSpace(strings.ToLower(ext))
	if e == "" {
		return ""
	}
	if !strings.HasPrefix(e, ".") {
		e = "." + e
	}
	return e
}

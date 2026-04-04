package rag

import (
	"encoding/json"
	"fmt"
)

// ReadDocumentPlaceholder returns the exact channel placeholder for a file name.
func ReadDocumentPlaceholder(fileName string) string {
	return fmt.Sprintf("[File: %s — use read_document tool to analyze this file]", fileName)
}

// EnrichOtherConfigWithRAG writes rag_indexing.supported_types when RAG is enabled.
// Returns the merged JSON, a deps report when rag_indexing was touched, and whether rag_indexing exists with enabled true.
func EnrichOtherConfigWithRAG(raw []byte) ([]byte, *DepsReport, error) {
	if len(raw) == 0 {
		return raw, nil, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, nil, err
	}
	riRaw, ok := root["rag_indexing"]
	if !ok || len(riRaw) == 0 {
		return raw, nil, nil
	}
	var ri struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(riRaw, &ri); err != nil {
		return nil, nil, err
	}
	if !ri.Enabled {
		var riMap map[string]any
		if err := json.Unmarshal(riRaw, &riMap); err != nil {
			return nil, nil, err
		}
		if riMap == nil {
			riMap = map[string]any{}
		}
		delete(riMap, "supported_types")
		riMap["enabled"] = false
		outRI, err := json.Marshal(riMap)
		if err != nil {
			return nil, nil, err
		}
		root["rag_indexing"] = outRI
		out, err := json.Marshal(root)
		if err != nil {
			return nil, nil, err
		}
		return out, nil, nil
	}
	report := CheckDeps()
	var riMap map[string]any
	if err := json.Unmarshal(riRaw, &riMap); err != nil {
		return nil, nil, err
	}
	riMap["supported_types"] = report.Supported
	outRI, err := json.Marshal(riMap)
	if err != nil {
		return nil, nil, err
	}
	root["rag_indexing"] = outRI
	out, err := json.Marshal(root)
	if err != nil {
		return nil, nil, err
	}
	return out, &report, nil
}

package methods

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// RAGMethods provides WS RPC methods for RAG management + upload.
// v1 is tenant+user scoped: all operations are restricted to the current session's
// {tenant_id, agent_id, user_id} (except owner role can delete across user_id).
type RAGMethods struct {
	ragStore           store.RAGStore
	attachmentHandler  channels.AttachmentHandler
}

func NewRAGMethods(ragStore store.RAGStore, ah channels.AttachmentHandler) *RAGMethods {
	return &RAGMethods{ragStore: ragStore, attachmentHandler: ah}
}

func (m *RAGMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodRAGUpload, m.handleUpload)
	router.Register(protocol.MethodRAGList, m.handleList)
	router.Register(protocol.MethodRAGDelete, m.handleDelete)
	// MethodRAGReindex is intentionally stubbed in v1 (wire later).
	router.Register(protocol.MethodRAGReindex, m.handleReindex)
}

type ragUploadParams struct {
	AgentID   string `json:"agentId"`
	FileName  string `json:"fileName"`
	MimeType  string `json:"mimeType"`
	Data      string `json:"data"` // base64
}

func (m *RAGMethods) handleUpload(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.attachmentHandler == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnavailable, "rag.upload not available"))
		return
	}
	var p ragUploadParams
	if err := json.Unmarshal(req.Params, &p); err != nil || p.Data == "" || p.FileName == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "fileName and data are required"))
		return
	}

	// Verify agent ID matches context (caller cannot upload to a different agent).
	ctxAgent := store.AgentIDFromContext(ctx)
	if ctxAgent == uuid.Nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, "agent_id required"))
		return
	}
	if p.AgentID != "" && p.AgentID != ctxAgent.String() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, "agentId mismatch"))
		return
	}

	// Base64 decode with size guard before allocating.
	// Default aligns with plan: 20MB.
	const maxBytes = 20 * 1024 * 1024
	decodedLen := base64.StdEncoding.DecodedLen(len(p.Data))
	if decodedLen > maxBytes {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, fmt.Sprintf("file too large (max %d bytes)", maxBytes)))
		return
	}
	raw, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid base64 data"))
		return
	}
	if len(raw) > maxBytes {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, fmt.Sprintf("file too large (max %d bytes)", maxBytes)))
		return
	}

	att := channels.Attachment{
		Bytes:    raw,
		MimeType: p.MimeType,
		FileName: p.FileName,
		Size:     len(raw),
	}
	sess := channels.SessionInfo{
		SessionKey: tools.ToolSessionKeyFromCtx(ctx),
	}
	msg, err := m.attachmentHandler.HandleAttachment(ctx, att, sess)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, err.Error()))
		return
	}

	// Handler messages are user-facing; keep a stable machine response too.
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"status":  "indexed",
		"message": msg,
	}))
}

type ragListParams struct {
	AgentID     string `json:"agentId"`
	SourceType  string `json:"sourceType,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (m *RAGMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ragStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnavailable, "rag.list not available"))
		return
	}
	var p ragListParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	// Enforce agent from ctx.
	ctxAgent := store.AgentIDFromContext(ctx)
	if ctxAgent == uuid.Nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, "agent_id required"))
		return
	}
	if p.AgentID != "" && p.AgentID != ctxAgent.String() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, "agentId mismatch"))
		return
	}

	docs, err := m.ragStore.ListDocuments(ctx, "", "", p.SourceType, p.Limit)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"documents": docs,
		"count":     len(docs),
	}))
}

type ragDeleteParams struct {
	AgentID string `json:"agentId"`
	DocID   string `json:"docId"`
}

func (m *RAGMethods) handleDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ragStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnavailable, "rag.delete not available"))
		return
	}
	var p ragDeleteParams
	if err := json.Unmarshal(req.Params, &p); err != nil || p.DocID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "docId required"))
		return
	}

	ctxAgent := store.AgentIDFromContext(ctx)
	if ctxAgent == uuid.Nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, "agent_id required"))
		return
	}
	if p.AgentID != "" && p.AgentID != ctxAgent.String() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, "agentId mismatch"))
		return
	}

	if err := m.ragStore.DeleteDocument(ctx, "", "", p.DocID); err != nil {
		if err == store.ErrNotFound || err == store.ErrForbidden {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "document not found"))
			return
		}
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": "deleted"}))
}

func (m *RAGMethods) handleReindex(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnavailable, "rag.reindex not implemented yet"))
}


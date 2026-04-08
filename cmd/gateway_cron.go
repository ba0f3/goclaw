package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/scheduler"
	"github.com/nextlevelbuilder/goclaw/internal/sessions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// makeCronJobHandler creates a cron job handler that routes through the scheduler's cron lane.
// This ensures per-session concurrency control (same job can't run concurrently)
// and integration with /stop, /stopall commands.
// cronHeartbeatWakeFn holds the heartbeat wake function, set after ticker creation.
// Safe because cron jobs only fire after Start(), well after this is set.
var cronHeartbeatWakeFn func(agentID string)

func makeCronJobHandler(sched *scheduler.Scheduler, msgBus *bus.MessageBus, cfg *config.Config, channelMgr *channels.Manager, sessionMgr store.SessionStore, agentStore store.AgentStore) func(job *store.CronJob) (*store.CronJobResult, error) {
	return func(job *store.CronJob) (*store.CronJobResult, error) {
		agentID := job.AgentID
		if agentID == "" && agentStore != nil {
			// Resolve real default agent from DB instead of using literal "default" string.
			tenantCtx := store.WithTenantID(context.Background(), job.TenantID)
			if defaultAgent, err := agentStore.GetDefault(tenantCtx); err == nil {
				agentID = defaultAgent.AgentKey
			} else {
				agentID = cfg.ResolveDefaultAgentID()
			}
		} else if agentID == "" {
			agentID = cfg.ResolveDefaultAgentID()
		} else if id, err := uuid.Parse(agentID); err == nil && agentStore != nil {
			// Resolve agentKey from UUID so session key uses agentKey
			// (consistent with chat/WS/team paths, fixes cache invalidation mismatch).
			cronCtx := store.WithTenantID(context.Background(), job.TenantID)
			if ag, err := agentStore.GetByID(cronCtx, id); err == nil {
				agentID = ag.AgentKey
			}
		} else {
			agentID = config.NormalizeAgentID(agentID)
		}

		sessionKey := sessions.BuildCronSessionKey(agentID, job.ID)
		channel := job.DeliverChannel
		if channel == "" {
			channel = "cron"
		}

		// Infer peer kind from the stored session metadata (group chats need it
		// so that tools like message can route correctly via group APIs).
		peerKind := resolveCronPeerKind(job)

		// Resolve channel type for system prompt context.
		channelType := resolveChannelType(channelMgr, channel)

		cronCtx := store.WithTenantID(context.Background(), job.TenantID)

		// Build cron context so the agent knows delivery target and requester.
		// When delivery targets a channel chat, inject recent history from that chat's
		// session (same key as inbound messages). Cron runs use a separate cron:* session,
		// so without this the model only sees the job payload — not group chat history.
		var extraPrompt string
		if job.Deliver && job.DeliverChannel != "" && job.DeliverTo != "" {
			linked := linkedDeliveryChatContextBlock(cronCtx, sessionMgr, agentID, job, peerKind)
			extraPrompt = fmt.Sprintf(
				"[Cron Job]\nThis is scheduled job \"%s\" (ID: %s).\n"+
					"Requester: user %s on channel \"%s\" (chat %s).\n"+
					"Your response will be automatically delivered to that chat — just produce the content directly.%s",
				job.Name, job.ID, job.UserID, job.DeliverChannel, job.DeliverTo, linked,
			)
		} else {
			extraPrompt = fmt.Sprintf(
				"[Cron Job]\nThis is scheduled job \"%s\" (ID: %s), created by user %s.\n"+
					"Delivery is not configured — respond normally.",
				job.Name, job.ID, job.UserID,
			)
		}

		// Reset session before each cron run to prevent tool errors from previous
		// runs from polluting the context and blocking future executions (#294).
		// Save() persists the empty session to DB so stale data won't reload after restart.
		// Stateless jobs skip this — they intentionally carry no session history.
		if !job.Stateless {
			sessionMgr.Reset(cronCtx, sessionKey)
			sessionMgr.Save(cronCtx, sessionKey)
		}

		// Schedule through cron lane — scheduler handles agent resolution and concurrency
		outCh := sched.Schedule(cronCtx, scheduler.LaneCron, agent.RunRequest{
			SessionKey:        sessionKey,
			Message:           job.Payload.Message,
			Channel:           channel,
			ChannelType:       channelType,
			ChatID:            job.DeliverTo,
			PeerKind:          peerKind,
			UserID:            job.UserID,
			RunID:             fmt.Sprintf("cron:%s", job.ID),
			Stream:            false,
			ExtraSystemPrompt: extraPrompt,
			TraceName:         fmt.Sprintf("Cron [%s] - %s", job.Name, agentID),
			TraceTags:         []string{"cron"},
		})

		// Block until the scheduled run completes
		outcome := <-outCh
		if outcome.Err != nil {
			return nil, outcome.Err
		}

		result := outcome.Result

		// If job wants delivery to a channel, send the agent response to the target chat.
		if job.Deliver && job.DeliverChannel != "" && job.DeliverTo != "" {
			outMsg := bus.OutboundMessage{
				Channel: job.DeliverChannel,
				ChatID:  job.DeliverTo,
				Content: result.Content,
			}
			if peerKind == "group" {
				outMsg.Metadata = map[string]string{"group_id": job.DeliverTo}
			}
			appendMediaToOutbound(&outMsg, result.Media)
			msgBus.PublishOutbound(outMsg)
		} else if job.Deliver {
			slog.Warn("cron: delivery configured but channel/chatID missing — output discarded",
				"job_id", job.ID, "job_name", job.Name, "channel", job.DeliverChannel, "to", job.DeliverTo)
		}

		cronResult := &store.CronJobResult{
			Content: result.Content,
		}
		if result.Usage != nil {
			cronResult.InputTokens = result.Usage.PromptTokens
			cronResult.OutputTokens = result.Usage.CompletionTokens
		}

		// wakeMode: trigger heartbeat after cron job completes.
		// Use original job.AgentID (UUID) — cronHeartbeatWakeFn expects UUID for ticker.Wake().
		if job.WakeHeartbeat && cronHeartbeatWakeFn != nil {
			cronHeartbeatWakeFn(job.AgentID)
		}

		return cronResult, nil
	}
}

// resolveCronPeerKind infers peer kind from the cron job's user ID.
// Group cron jobs have userID prefixed with "group:" or "guild:" (set during job creation).
// Telegram supergroups/channels use negative chat IDs starting with -100 — treat as group when
// deliver target looks like one (API-created jobs may omit group-scoped user IDs).
func resolveCronPeerKind(job *store.CronJob) string {
	if strings.HasPrefix(job.UserID, "group:") || strings.HasPrefix(job.UserID, "guild:") {
		return "group"
	}
	if strings.HasPrefix(job.DeliverTo, "-100") {
		return "group"
	}
	return ""
}

const (
	linkedChatMaxMessages     = 35
	linkedChatMaxCharsPerMsg  = 2500
	linkedChatMaxTotalBytes   = 48 * 1024
)

// linkedDeliveryChatContextBlock returns text to append to the cron extra system prompt:
// recent messages from the delivery chat's normal session (where @mentions and worklogs land).
// Empty string if there is no history or session store.
func linkedDeliveryChatContextBlock(ctx context.Context, sessionMgr store.SessionStore, agentID string, job *store.CronJob, peerKind string) string {
	if sessionMgr == nil || job.DeliverChannel == "" || job.DeliverTo == "" {
		return ""
	}
	var pk sessions.PeerKind
	if peerKind == "group" {
		pk = sessions.PeerGroup
	} else {
		pk = sessions.PeerDirect
	}
	chatKey := sessions.BuildScopedSessionKey(agentID, job.DeliverChannel, pk, job.DeliverTo)
	history := sessionMgr.GetHistory(ctx, chatKey)
	if len(history) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Recent messages from this chat session — session_key ")
	b.WriteString(chatKey)
	b.WriteString("]\n")
	// Same visibility rules as sessions_history (no raw tool I/O).
	var lines []string
	for _, m := range history {
		if m.Role == "tool" {
			continue
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 && strings.TrimSpace(m.Content) == "" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if utf8.RuneCountInString(content) > linkedChatMaxCharsPerMsg {
			runes := []rune(content)
			content = string(runes[:linkedChatMaxCharsPerMsg]) + "... [truncated]"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", m.Role, content))
	}
	if len(lines) > linkedChatMaxMessages {
		lines = lines[len(lines)-linkedChatMaxMessages:]
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	out := b.String()
	if len(out) > linkedChatMaxTotalBytes {
		out = out[:linkedChatMaxTotalBytes]
		for len(out) > 0 && !utf8.ValidString(out) {
			out = out[:len(out)-1]
		}
		out += "\n... [chat context truncated]"
	}
	return out
}

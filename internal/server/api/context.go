package api

import (
	"context"
	"net/http"
)

type contextKey string

const contextKeyAgentID contextKey = "agent_id"

func contextWithAgentID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyAgentID, id)
}

func agentIDFromContext(r *http.Request) string {
	if v := r.Context().Value(contextKeyAgentID); v != nil {
		return v.(string)
	}
	return ""
}

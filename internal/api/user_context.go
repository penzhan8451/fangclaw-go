package api

import (
	"context"
	"net/http"

	"github.com/penzhan8451/fangclaw-go/internal/kernel"
)

func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

func getUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value("username").(string); ok {
		return username
	}
	return ""
}

func getUserKernelFromContext(ctx context.Context) *kernel.Kernel {
	if k, ok := ctx.Value("user_kernel").(*kernel.Kernel); ok {
		return k
	}
	return nil
}

func requireUserContext(r *http.Request) (userID string, userKernel *kernel.Kernel, ok bool) {
	ctx := r.Context()
	userID = getUserIDFromContext(ctx)
	userKernel = getUserKernelFromContext(ctx)

	if userID == "" || userKernel == nil {
		return "", nil, false
	}
	return userID, userKernel, true
}

func respondUnauthorized(w http.ResponseWriter, message string) {
	WriteJSON(w, http.StatusUnauthorized, map[string]string{
		"error": message,
	})
}

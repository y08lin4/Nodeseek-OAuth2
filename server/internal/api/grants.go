package api

import (
	"net/http"
)

// handleGrants GET /api/grants（当前用户的授权列表，join 应用展示字段）。
func (a *API) handleGrants(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	gs, err := a.store.GetGrants(sess.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取授权列表失败")
		return
	}
	grants := []map[string]any{}
	for i := range gs {
		item := map[string]any{
			"user_id":    gs[i].UserID,
			"client_id":  gs[i].ClientID,
			"granted_at": gs[i].GrantedAt,
			"status":     gs[i].Status,
		}
		// join client 展示字段（应用被删则回退空值）。
		if c, err := a.store.GetClient(gs[i].ClientID); err == nil && c != nil {
			item["client_name"] = c.ClientName
			item["icon_url"] = c.IconURL
			item["min_rank"] = c.MinRank
		} else {
			item["client_name"] = ""
			item["icon_url"] = ""
			item["min_rank"] = 0
		}
		grants = append(grants, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "grants": grants})
}

// handleGrantRevoke POST /api/grants/{client_id}/revoke（撤销授权并作废该 user+client 的 token）。
func (a *API) handleGrantRevoke(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	clientID := r.PathValue("client_id")
	if err := a.store.RevokeGrant(sess.UserID, clientID); err != nil {
		writeError(w, http.StatusInternalServerError, "撤销授权失败")
		return
	}
	if err := a.store.DeleteTokensFor(sess.UserID, clientID); err != nil {
		writeError(w, http.StatusInternalServerError, "作废 token 失败")
		return
	}
	a.audit.Eventf("grant.revoke", remoteIP(r), sess.UserID, clientID, "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

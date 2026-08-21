package api

import (
	"net/http"

	"nodeseek-oauth2/server/internal/store"
)

// isValidClientStatus 判断是否为合法的应用状态枚举。
func isValidClientStatus(s string) bool {
	switch s {
	case "approved", "pending_review", "rejected", "paused", "pause_request", "resume_request", "delete_request":
		return true
	}
	return false
}

// reviewType 返回待审核状态对应的审核类型；非待审核状态返回空串。
func reviewType(status string) string {
	switch status {
	case "pending_review":
		return "app"
	case "pause_request":
		return "pause"
	case "resume_request":
		return "resume"
	case "delete_request":
		return "delete"
	}
	return ""
}

// handleClientPause POST /api/client/{client_id}/pause（owner 申请暂停）。
func (a *API) handleClientPause(w http.ResponseWriter, r *http.Request) {
	a.clientStatusRequest(w, r, "approved", "pause_request", "仅已通过应用可申请暂停", "client.pause_request")
}

// handleClientResume POST /api/client/{client_id}/resume（owner 申请恢复）。
func (a *API) handleClientResume(w http.ResponseWriter, r *http.Request) {
	a.clientStatusRequest(w, r, "paused", "resume_request", "仅已暂停应用可申请恢复", "client.resume_request")
}

// handleClientDeleteRequest POST /api/client/{client_id}/delete-request（owner 申请删除）。
func (a *API) handleClientDeleteRequest(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	clientID := r.PathValue("client_id")
	client, err := a.store.GetClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	if client.OwnerUserID != sess.UserID {
		writeError(w, http.StatusForbidden, "无权操作该应用")
		return
	}
	if client.Builtin {
		writeError(w, http.StatusForbidden, "内置应用不可操作")
		return
	}
	if client.Status != "approved" && client.Status != "paused" {
		writeError(w, http.StatusForbidden, "仅已通过或已暂停应用可申请删除")
		return
	}
	prev := client.Status
	updated, err := a.store.UpdateClient(clientID, func(c *store.Client) {
		c.Status = "delete_request"
		c.PrevStatus = prev
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新应用失败")
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	a.audit.Eventf("client.delete_request", remoteIP(r), sess.UserID, clientID, "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": updated.Status})
}

// clientStatusRequest 处理 pause/resume 这类「owner + 指定状态 → 申请态」的通用逻辑。
func (a *API) clientStatusRequest(w http.ResponseWriter, r *http.Request, from, to, errMsg, auditEvent string) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	clientID := r.PathValue("client_id")
	client, err := a.store.GetClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	if client.OwnerUserID != sess.UserID {
		writeError(w, http.StatusForbidden, "无权操作该应用")
		return
	}
	if client.Builtin {
		writeError(w, http.StatusForbidden, "内置应用不可操作")
		return
	}
	if client.Status != from {
		writeError(w, http.StatusForbidden, errMsg)
		return
	}
	updated, err := a.store.UpdateClient(clientID, func(c *store.Client) {
		c.Status = to
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新应用失败")
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	a.audit.Eventf(auditEvent, remoteIP(r), sess.UserID, clientID, "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": updated.Status})
}

// handleAdminReviews GET /api/admin/reviews（待审核队列）。
func (a *API) handleAdminReviews(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	cs, err := a.store.ListClients()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取应用列表失败")
		return
	}
	reviews := []map[string]any{}
	for i := range cs {
		typ := reviewType(cs[i].Status)
		if typ == "" {
			continue
		}
		reviews = append(reviews, map[string]any{
			"type":          typ,
			"client_id":     cs[i].ClientID,
			"client_name":   cs[i].ClientName,
			"owner_user_id": cs[i].OwnerUserID,
			"detail":        "",
			"created_at":    cs[i].CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "reviews": reviews})
}

// handleAdminReview POST /api/admin/review（审核：通过/拒绝）。
func (a *API) handleAdminReview(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	var req struct {
		Type     string `json:"type"`
		ClientID string `json:"client_id"`
		Action   string `json:"action"`
		Reason   string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if req.Action != "approve" && req.Action != "reject" {
		writeError(w, http.StatusUnprocessableEntity, "action 必须为 approve 或 reject")
		return
	}
	client, err := a.store.GetClient(req.ClientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}

	ip := remoteIP(r)
	approve := req.Action == "approve"

	switch req.Type {
	case "app":
		if client.Status != "pending_review" {
			writeError(w, http.StatusBadRequest, "当前应用不在待审核状态")
			return
		}
		target := "rejected"
		if approve {
			target = "approved"
		}
		a.applyReview(w, req.ClientID, target, approve, req.Reason, ip)
		// 审核结果邮件：申请者提交了通知邮箱则发（失败不阻塞审核）。
		if client.NotifyEmail != "" && client.NotifyEnabled {
			a.sendReviewResultMail(*client, approve, req.Reason)
		}
	case "pause":
		if client.Status != "pause_request" {
			writeError(w, http.StatusBadRequest, "当前应用不在暂停申请状态")
			return
		}
		target := "approved"
		if approve {
			target = "paused"
		}
		a.applyReview(w, req.ClientID, target, approve, req.Reason, ip)
	case "resume":
		if client.Status != "resume_request" {
			writeError(w, http.StatusBadRequest, "当前应用不在恢复申请状态")
			return
		}
		target := "paused"
		if approve {
			target = "approved"
		}
		a.applyReview(w, req.ClientID, target, approve, req.Reason, ip)
	case "delete":
		if client.Status != "delete_request" {
			writeError(w, http.StatusBadRequest, "当前应用不在删除申请状态")
			return
		}
		if approve {
			if _, err := a.store.DeleteClient(req.ClientID); err != nil {
				writeError(w, http.StatusInternalServerError, "删除应用失败")
				return
			}
			a.audit.Eventf("review.approve", ip, "", req.ClientID, req.Reason)
			writeJSON(w, http.StatusOK, map[string]any{"success": true})
			return
		}
		// reject 回原状态（delete_request 前为 approved 或 paused）。
		target := client.PrevStatus
		if target == "" {
			target = "approved"
		}
		a.applyReview(w, req.ClientID, target, false, req.Reason, ip)
	default:
		writeError(w, http.StatusUnprocessableEntity, "未知的审核类型")
	}
}

// applyReview 应用审核结果（更新 status 并记审计 review.approve/review.reject）。
func (a *API) applyReview(w http.ResponseWriter, clientID, target string, approve bool, reason, ip string) {
	updated, err := a.store.UpdateClient(clientID, func(c *store.Client) {
		c.Status = target
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新应用失败")
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	if approve {
		a.audit.Eventf("review.approve", ip, "", clientID, reason)
	} else {
		a.audit.Eventf("review.reject", ip, "", clientID, reason)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "client": clientOwnerJSON(*updated)})
}

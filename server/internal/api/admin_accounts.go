package api

import (
	"net/http"

	"nodeseek-oauth2/server/internal/store"
)

// accountJSON 构造账号列表项（不含 Cookie 密文）。
func accountJSON(ac store.Account) map[string]any {
	return map[string]any{
		"account_id":    ac.AccountID,
		"account_name":  ac.AccountName,
		"priority":      ac.Priority,
		"enabled":       ac.Enabled,
		"updated_at":    ac.UpdatedAt,
		"last_error":    ac.LastError,
		"fail_count":    ac.FailCount,
		"auto_detected": ac.AutoDetected,
	}
}

// handleAdminAccountsList GET /api/admin/accounts（列表，不含 Cookie）。
func (a *API) handleAdminAccountsList(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	accounts, err := a.store.ListAccounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取账号列表失败")
		return
	}
	list := []map[string]any{}
	for _, ac := range accounts {
		list = append(list, accountJSON(ac))
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "accounts": list})
}

// handleAdminAccountCreate POST /api/admin/accounts。
func (a *API) handleAdminAccountCreate(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	var req struct {
		AccountID   string `json:"account_id"`
		AccountName string `json:"account_name"`
		Priority    *int   `json:"priority"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if req.AccountID == "" || !isNumeric(req.AccountID) {
		writeError(w, http.StatusUnprocessableEntity, "account_id 必须为纯数字")
		return
	}
	// account_id 唯一校验。
	if existing, err := a.store.GetAccount(req.AccountID); err != nil {
		writeError(w, http.StatusInternalServerError, "读取账号失败")
		return
	} else if existing != nil {
		writeError(w, http.StatusUnprocessableEntity, "account_id 已存在")
		return
	}
	ac := store.Account{
		AccountID:    req.AccountID,
		AccountName:  req.AccountName,
		Priority:     0,
		Enabled:      true,
		AutoDetected: false,
	}
	if req.Priority != nil {
		ac.Priority = *req.Priority
	}
	if req.Enabled != nil {
		ac.Enabled = *req.Enabled
	}
	if err := a.store.AddAccount(ac); err != nil {
		writeError(w, http.StatusInternalServerError, "创建账号失败")
		return
	}
	a.audit.Eventf("admin.account.create", remoteIP(r), "", "", req.AccountID)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "account": accountJSON(ac)})
}

// handleAdminAccountPatch PATCH /api/admin/accounts/{account_id}（调整 priority/enabled）。
func (a *API) handleAdminAccountPatch(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	id := r.PathValue("account_id")
	var req struct {
		Priority *int  `json:"priority"`
		Enabled  *bool `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	updated, err := a.store.UpdateAccount(id, func(ac *store.Account) {
		if req.Priority != nil {
			ac.Priority = *req.Priority
		}
		if req.Enabled != nil {
			ac.Enabled = *req.Enabled
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新账号失败")
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "账号不存在")
		return
	}
	a.audit.Eventf("admin.account.patch", remoteIP(r), "", "", id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "account": accountJSON(*updated)})
}

// handleAdminAccountDelete DELETE /api/admin/accounts/{account_id}（至少保留 1 个）。
func (a *API) handleAdminAccountDelete(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	id := r.PathValue("account_id")
	count, err := a.store.CountAccounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取账号失败")
		return
	}
	if count <= 1 {
		writeError(w, http.StatusBadRequest, "至少保留一个系统账号")
		return
	}
	deleted, err := a.store.DeleteAccount(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "删除账号失败")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "账号不存在")
		return
	}
	a.audit.Eventf("admin.account.delete", remoteIP(r), "", "", id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"mikrotik-monitor/internal/alerter"
	"mikrotik-monitor/internal/auth"
	"mikrotik-monitor/internal/mikrotik"
	"mikrotik-monitor/internal/models"
	"mikrotik-monitor/internal/poller"
)

type Server struct {
	DB      *models.DB
	Auth    *auth.Manager
	Poller  *poller.Manager
	Alerter *alerter.Engine
}

func (s *Server) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/auth/login", s.handleLogin)
	r.Post("/auth/logout", s.handleLogout)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Group(func(r chi.Router) {
		r.Use(s.Auth.Middleware)
		r.Get("/auth/me", s.handleMe)
		r.Get("/auth/sessions", s.handleListSessions)
		r.Post("/auth/change-password", s.handleChangePassword)

		r.Get("/dashboard", s.dashboardOverview)
		r.Get("/devices", s.listDevices)
		r.Post("/devices", s.createDevice)
		r.Post("/devices/preview-discover", s.previewDiscover)
		r.Get("/devices/{id}", s.getDevice)
		r.Put("/devices/{id}", s.updateDevice)
		r.Delete("/devices/{id}", s.deleteDevice)
		r.Post("/devices/{id}/copy", s.copyDevice)
		r.Post("/devices/{id}/test", s.testDevice)
		r.Get("/devices/{id}/discover", s.discoverInterfaces)
		r.Put("/devices/{id}/interfaces", s.setInterfaces)
		r.Get("/devices/{id}/interfaces", s.listInterfaces)
		r.Get("/devices/{id}/history", s.getHistory)

		r.Get("/alert-rules", s.listRules)
		r.Post("/alert-rules", s.createRule)
		r.Put("/alert-rules/{id}", s.updateRule)
		r.Delete("/alert-rules/{id}", s.deleteRule)

		r.Get("/alert-history", s.listAlertHistory)
		r.Get("/settings/notification", s.getNotification)
		r.Get("/settings/app", s.getAppSettings)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Put("/settings/notification", s.updateNotification)
			r.Put("/settings/app", s.updateAppSettings)
			r.Post("/settings/notification/test", s.testNotification)
			r.Get("/users", s.listUsers)
			r.Post("/users", s.createUser)
			r.Delete("/users/{id}", s.deleteUser)
			r.Put("/users/{id}/role", s.updateUserRole)
		})
	})

	return r
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	u, err := s.DB.GetUserByUsername(req.Username)
	if err != nil || !models.CheckPasswordHash(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.Auth.CreateSession(r, u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session error")
		return
	}
	auth.SetTokenCookie(w, token, 86400)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":                token,
		"id":                   u.ID,
		"username":             u.Username,
		"role":                 u.Role,
		"must_change_password": u.MustChangePassword,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.ExtractToken(r)
	_ = s.Auth.RevokeToken(token)
	auth.ClearTokenCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	c := auth.ClaimsFromContext(r.Context())
	sessions, err := s.DB.ListUserSessions(c.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type row struct {
		models.Session
		Current bool `json:"current"`
	}
	var out []row
	for _, sess := range sessions {
		out = append(out, row{Session: sess, Current: sess.ID == c.SessionID})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	c := auth.ClaimsFromContext(r.Context())
	u, err := s.DB.GetUserByID(c.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":                   u.ID,
		"username":             u.Username,
		"role":                 u.Role,
		"must_change_password": u.MustChangePassword,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	c := auth.ClaimsFromContext(r.Context())
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Password) < 4 {
		writeError(w, http.StatusBadRequest, "password required")
		return
	}
	if err := s.DB.UpdateUserPassword(c.UserID, req.Password, true); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Keep this browser logged in; revoke other sessions for security.
	token := auth.ExtractToken(r)
	_ = s.DB.RevokeOtherUserSessions(c.UserID, token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) dashboardOverview(w http.ResponseWriter, r *http.Request) {
	devs, err := s.DB.ListDevices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type ifaceLive struct {
		InterfaceName string `json:"interface_name"`
		InterfaceType string `json:"interface_type"`
		TxBps         int64  `json:"tx_bps"`
		RxBps         int64  `json:"rx_bps"`
	}
	type row struct {
		models.Device
		Interfaces []ifaceLive `json:"interfaces"`
	}
	out := make([]row, 0, len(devs))
	for _, d := range devs {
		d.Online = s.Poller.IsOnline(d.ID)
		d.LastError = s.Poller.LastError(d.ID)
		ifaces, _ := s.DB.ListMonitoredInterfaces(d.ID)
		latest := s.Poller.LatestSamples(d.ID)
		byName := make(map[string]models.TrafficSample)
		for _, s := range latest {
			byName[s.InterfaceName] = s
		}
		live := make([]ifaceLive, 0, len(ifaces))
		for _, i := range ifaces {
			if !i.Enabled {
				continue
			}
			s := byName[i.InterfaceName]
			live = append(live, ifaceLive{
				InterfaceName: i.InterfaceName,
				InterfaceType: i.InterfaceType,
				TxBps:         s.TxBps,
				RxBps:         s.RxBps,
			})
		}
		if live == nil {
			live = []ifaceLive{}
		}
		out = append(out, row{Device: d, Interfaces: live})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	devs, err := s.DB.ListDevices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range devs {
		devs[i].Online = s.Poller.IsOnline(devs[i].ID)
		devs[i].LastError = s.Poller.LastError(devs[i].ID)
	}
	writeJSON(w, http.StatusOK, devs)
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	dev, err := s.DB.GetDevice(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	dev.Online = s.Poller.IsOnline(dev.ID)
	dev.LastError = s.Poller.LastError(dev.ID)
	writeJSON(w, http.StatusOK, dev)
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var in models.DeviceInput
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	dev, err := s.DB.CreateDevice(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Poller starts when interfaces are saved (setInterfaces) or via sync loop.
	writeJSON(w, http.StatusCreated, dev)
}

func (s *Server) updateDevice(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var in models.DeviceInput
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	dev, err := s.DB.UpdateDevice(id, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Poller.ReloadDevice(id)
	writeJSON(w, http.StatusOK, dev)
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.DB.DeleteDevice(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Poller.ReloadDevice(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) copyDevice(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Host string `json:"host"`
		Name string `json:"name"`
	}
	_ = readJSON(r, &req)
	dev, err := s.DB.CopyDevice(id, req.Host, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Poller.ReloadDevice(dev.ID)
	writeJSON(w, http.StatusCreated, dev)
}

func (s *Server) testDevice(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var in models.DeviceInput
	if r.ContentLength > 0 {
		_ = readJSON(r, &in)
	}
	var host, user, pw string
	var port int
	if in.Host != "" {
		host, user, pw, port = in.Host, in.Username, in.Password, in.Port
	} else {
		dev, err := s.DB.GetDevice(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		p, err := s.DB.GetDevicePassword(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		host, user, pw, port = dev.Host, dev.Username, p, dev.Port
		if in.Password != "" {
			pw = in.Password
		}
	}
	if port == 0 {
		port = 8728
	}
	c := &mikrotik.Client{Host: host, Port: port, Username: user, Password: pw}
	if err := c.TestConnection(); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) previewDiscover(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var in struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Port == 0 {
		in.Port = 8728
	}
	c := &mikrotik.Client{Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password}
	list, err := c.ListInterfaces()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	grouped := make(map[string][]models.DiscoveredInterface)
	for _, i := range list {
		grouped[i.Type] = append(grouped[i.Type], i)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"interfaces": list, "grouped": grouped})
}

func (s *Server) discoverInterfaces(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	dev, err := s.DB.GetDevice(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	pw, err := s.DB.GetDevicePassword(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	c := &mikrotik.Client{Host: dev.Host, Port: dev.Port, Username: dev.Username, Password: pw}
	list, err := c.ListInterfaces()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	grouped := make(map[string][]models.DiscoveredInterface)
	for _, i := range list {
		grouped[i.Type] = append(grouped[i.Type], i)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"interfaces": list, "grouped": grouped})
}

func (s *Server) setInterfaces(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Interfaces []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"interfaces"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	names := make([]string, 0, len(req.Interfaces))
	types := make(map[string]string)
	for _, i := range req.Interfaces {
		names = append(names, i.Name)
		types[i.Name] = i.Type
	}
	if err := s.DB.SetMonitoredInterfaces(id, names, types); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Poller.ReloadDevice(id)
	list, _ := s.DB.ListMonitoredInterfaces(id)
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) listInterfaces(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	list, err := s.DB.ListMonitoredInterfaces(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) getHistory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	iface := r.URL.Query().Get("interface")
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 1
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	samples, err := s.DB.GetTrafficHistory(id, iface, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.DB.ListAlertRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	var in models.AlertRuleInput
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	rule, err := s.DB.CreateAlertRule(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var in models.AlertRuleInput
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	rule, err := s.DB.UpdateAlertRule(id, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	if auth.ClaimsFromContext(r.Context()).Role != "admin" {
		writeError(w, http.StatusForbidden, "admin only")
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.DB.DeleteAlertRule(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAlertHistory(w http.ResponseWriter, r *http.Request) {
	h, err := s.DB.ListAlertHistory(200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	c, err := s.DB.GetNotificationConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) updateNotification(w http.ResponseWriter, r *http.Request) {
	var c models.NotificationConfig
	if err := readJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.DB.UpdateNotificationConfig(c); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) getAppSettings(w http.ResponseWriter, r *http.Request) {
	s2, err := s.DB.GetAppSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"retention_days": s2.RetentionDays,
	})
}

func (s *Server) updateAppSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.RetentionDays < 1 {
		req.RetentionDays = 1
	}
	if err := s.DB.UpdateAppSettings(req.RetentionDays); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) testNotification(w http.ResponseWriter, r *http.Request) {
	if err := s.Alerter.SendTestNotification("MikroTik Monitor test notification"); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.DB.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type safe struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	var out []safe
	for _, u := range users {
		out = append(out, safe{u.ID, u.Username, u.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	u, err := s.DB.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": u.ID, "username": u.Username, "role": u.Role})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.DB.DeleteUser(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateUserRole(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Role string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.DB.UpdateUserRole(id, req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"shamal/service/internal/organization"
	"shamal/service/internal/permissions"
	"shamal/service/internal/platform"
)

func (a *API) orgStructure(w http.ResponseWriter, r *http.Request) {
	user, err := a.authorize(r, "organization", permissions.ActionView, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	nodes, err := a.Org.List(r.Context(), user.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	staff, err := a.Identity.ListStaff(r.Context(), user.CompanyID, r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	memberships, err := a.Org.ListMemberships(r.Context(), user.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	roles, err := a.Perm.NodeRules(r.Context(), user.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	roleViews := make([]permissions.RuleView, 0, len(roles))
	for _, rule := range roles {
		roleViews = append(roleViews, permissions.ViewRule(rule))
	}
	writeJSON(w, 200, map[string]any{
		"nodes": nodes, "staff": staff, "memberships": memberships, "roles": roleViews,
	})
}

func (a *API) createMembership(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID  string `json:"user_id"`
		NodeID  string `json:"node_id"`
		Primary bool   `json:"is_primary"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := uuid.Parse(body.UserID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	if _, err := uuid.Parse(body.NodeID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	actor, err := a.authorize(r, "organization", permissions.ActionCreate, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	person, err := a.Identity.FindByID(r.Context(), body.UserID)
	if err != nil || person.CompanyID != actor.CompanyID {
		writeErr(w, r, platform.E("not_found", 404))
		return
	}
	nodes, err := a.Org.List(r.Context(), actor.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var nodeOK bool
	for _, node := range nodes {
		if node.ID == body.NodeID && node.Status == "active" {
			nodeOK = true
		}
	}
	if !nodeOK {
		writeErr(w, r, platform.E("not_found", 404))
		return
	}
	current, err := a.Org.Memberships(r.Context(), body.UserID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	now := time.Now().UTC()
	for _, item := range current {
		if item.NodeID == body.NodeID && permissions.MembershipActive(permissions.Membership{NodeID: item.NodeID, Starts: item.StartsAt, Ends: item.EndsAt}, now) {
			writeErr(w, r, platform.E("validation", 422))
			return
		}
	}
	created, err := a.Org.AddMembership(r.Context(), organization.Membership{
		UserID: body.UserID, NodeID: body.NodeID, CompanyID: actor.CompanyID, Primary: body.Primary, StartsAt: now,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, created)
}

func (a *API) permissionAccount(w http.ResponseWriter, r *http.Request) {
	sess, user, err := a.sessionUser(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	targetID := r.URL.Query().Get("user_id")
	if targetID == "" {
		targetID = user.ID
	}
	if _, err := uuid.Parse(targetID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	target := user
	if targetID != user.ID {
		if _, err := a.authorize(r, "permissions", permissions.ActionView, nil); err != nil {
			writeErr(w, r, err)
			return
		}
		target, err = a.Identity.FindByID(r.Context(), targetID)
		if err != nil || target.CompanyID != user.CompanyID {
			writeErr(w, r, platform.E("not_found", 404))
			return
		}
	}
	account, err := a.permissionView(r, target.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	_ = sess
	writeJSON(w, 200, map[string]any{
		"user":   map[string]any{"id": target.ID, "display_name": target.DisplayName, "email": target.Email, "status": target.Status},
		"direct": account.Direct, "inherited": account.Inherited, "temporary": account.Temporary, "deny": account.Deny, "breaks": account.Breaks,
	})
}

func (a *API) permissionView(r *http.Request, userID string) (permissions.Account, error) {
	memberships, err := a.Org.Memberships(r.Context(), userID)
	if err != nil {
		return permissions.Account{}, err
	}
	if len(memberships) == 0 {
		return permissions.Present(nil, nil), nil
	}
	orgNodes, err := a.Org.List(r.Context(), memberships[0].CompanyID)
	if err != nil {
		return permissions.Account{}, err
	}
	permNodes := make([]permissions.Node, 0, len(orgNodes))
	nodeIDs := make([]string, 0, len(orgNodes))
	for _, node := range orgNodes {
		permNodes = append(permNodes, permissions.Node{ID: node.ID, ParentID: node.ParentID})
		nodeIDs = append(nodeIDs, node.ID)
	}
	active := make([]permissions.Membership, 0)
	pathIDs := make([]string, 0)
	now := time.Now().UTC()
	for _, item := range memberships {
		active = append(active, permissions.Membership{NodeID: item.NodeID, Starts: item.StartsAt, Ends: item.EndsAt})
		if permissions.MembershipActive(permissions.Membership{NodeID: item.NodeID, Starts: item.StartsAt, Ends: item.EndsAt}, now) {
			pathIDs = append(pathIDs, permissions.AncestorIDs(permNodes, item.NodeID)...)
		}
	}
	rules, breaksUsed, err := a.Perm.Load(r.Context(), userID, nodeIDs)
	if err != nil {
		return permissions.Account{}, err
	}
	collected := permissions.Collect(userID, permNodes, active, rules, breaksUsed, now)
	breaks, err := a.Perm.BreakViews(r.Context(), pathIDs)
	if err != nil {
		return permissions.Account{}, err
	}
	return permissions.Present(collected, breaks), nil
}

func (a *API) createGrant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID    string `json:"user_id"`
		Domain    string `json:"domain"`
		Action    string `json:"action"`
		Effect    string `json:"effect"`
		ScopeKind string `json:"scope_kind"`
		ScopeNode string `json:"scope_node_id"`
		EndsAt    string `json:"ends_at"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	actor, err := a.authorize(r, "permissions", permissions.ActionCreate, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if !knownDomain(body.Domain) || !knownAction(body.Action) || !knownScope(body.ScopeKind) {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	if body.Effect != "allow" && body.Effect != "deny" {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	if _, err := uuid.Parse(body.UserID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	person, err := a.Identity.FindByID(r.Context(), body.UserID)
	if err != nil || person.CompanyID != actor.CompanyID {
		writeErr(w, r, platform.E("not_found", 404))
		return
	}
	var ends *time.Time
	if body.EndsAt != "" {
		parsed, err := time.Parse(time.RFC3339, body.EndsAt)
		if err != nil || !parsed.After(time.Now()) {
			writeErr(w, r, platform.E("validation", 422))
			return
		}
		ends = &parsed
	}
	scopeNode := body.ScopeNode
	if body.ScopeKind == string(permissions.ScopeCompany) && scopeNode == "" {
		scopeNode = actor.CompanyID
	}
	if scopeNode != "" {
		if _, err := uuid.Parse(scopeNode); err != nil {
			writeErr(w, r, platform.E("validation", 422))
			return
		}
	}
	layer := permissions.LayerDirect
	effect := permissions.EffectAllow
	if body.Effect == "deny" {
		layer = permissions.LayerExplicitDeny
		effect = permissions.EffectDeny
	}
	created, err := a.Perm.Create(r.Context(), actor.CompanyID, actor.ID, permissions.Rule{
		Domain: body.Domain, Action: permissions.Action(body.Action), Effect: effect, Layer: layer,
		Scope: permissions.Scope{Kind: permissions.ScopeKind(body.ScopeKind), NodeID: scopeNode}, EndsAt: ends,
		Source: "individual", GranteeID: body.UserID, AttachedTo: "user",
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	shown := permissions.Present([]permissions.Rule{created}, nil)
	writeJSON(w, 201, map[string]any{"direct": shown.Direct, "temporary": shown.Temporary, "deny": shown.Deny})
}

func (a *API) createBreak(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID string `json:"node_id"`
		Domain string `json:"domain"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Reason == "" || !knownDomain(body.Domain) || !knownAction(body.Action) {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	if _, err := uuid.Parse(body.NodeID); err != nil {
		writeErr(w, r, platform.E("validation", 422))
		return
	}
	actor, err := a.authorize(r, "permissions", permissions.ActionCreate, nil)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	nodes, err := a.Org.List(r.Context(), actor.CompanyID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	found := false
	for _, node := range nodes {
		if node.ID == body.NodeID {
			found = true
		}
	}
	if !found {
		writeErr(w, r, platform.E("not_found", 404))
		return
	}
	existing, err := a.Perm.BreakViews(r.Context(), []string{body.NodeID})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	for _, item := range existing {
		if item.Domain == body.Domain && item.Action == body.Action {
			writeErr(w, r, platform.E("validation", 422))
			return
		}
	}
	item := permissions.Break{NodeID: body.NodeID, Domain: body.Domain, Action: permissions.Action(body.Action)}
	if err := a.Perm.CreateBreak(r.Context(), actor.CompanyID, actor.ID, item, body.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"node_id": body.NodeID, "domain": body.Domain, "action": body.Action, "reason": body.Reason})
}

func knownDomain(value string) bool {
	switch value {
	case "geography", "catalog", "organization", "permissions", "files", "identity", "clients", "visits", "deals", "finance", "messages", "disputes", "reports":
		return true
	default:
		return false
	}
}

func knownAction(value string) bool {
	switch permissions.Action(value) {
	case permissions.ActionView, permissions.ActionCreate, permissions.ActionUpdate, permissions.ActionApprove, permissions.ActionPublish, permissions.ActionArchive, permissions.ActionDeleteDraft:
		return true
	default:
		return false
	}
}

func knownScope(value string) bool {
	switch permissions.ScopeKind(value) {
	case permissions.ScopeOwn, permissions.ScopeAssigned, permissions.ScopeTeam, permissions.ScopeDepartment, permissions.ScopeBranch, permissions.ScopeCompany:
		return true
	default:
		return false
	}
}

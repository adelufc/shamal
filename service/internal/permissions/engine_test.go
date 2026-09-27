package permissions

import (
	"testing"
	"time"
)

func activeSubject() Subject {
	return Subject{UserID: "user", CompanyID: "co", Status: "active", StepUp: true}
}

func companyAllow(layer Layer, action Action) Rule {
	return Rule{
		ID: "allow", Domain: "geography", Action: action, Effect: EffectAllow, Layer: layer,
		Scope: Scope{Kind: ScopeCompany, NodeID: "co"}, Source: "role",
	}
}

func TestInheritedAllowWithExplicitDeny(t *testing.T) {
	res := &Resource{CompanyID: "co", BranchID: "b1"}
	rules := []Rule{
		companyAllow(LayerInherited, ActionView),
		{
			ID: "deny-1", Domain: "geography", Action: ActionView, Effect: EffectDeny, Layer: LayerExplicitDeny,
			Scope: Scope{Kind: ScopeCompany, NodeID: "co"}, Source: "deny-rule",
		},
	}
	got := Decide(time.Now(), activeSubject(), "geography", ActionView, res, rules, false)
	if got.Allowed || got.Code != "explicit_deny" || got.DenySource != "deny-rule" {
		t.Fatalf("%+v", got)
	}
}

func TestExceptionReplacesNarrowerDirectGrant(t *testing.T) {
	res := &Resource{CompanyID: "co", BranchID: "b1", TeamID: "other-team"}
	rules := []Rule{
		{
			ID: "ex", Domain: "geography", Action: ActionUpdate, Effect: EffectAllow, Layer: LayerException,
			Scope: Scope{Kind: ScopeBranch, NodeID: "b1"},
		},
		{
			ID: "direct", Domain: "geography", Action: ActionUpdate, Effect: EffectAllow, Layer: LayerDirect,
			Scope: Scope{Kind: ScopeTeam, NodeID: "team-1"},
		},
	}
	got := Decide(time.Now(), activeSubject(), "geography", ActionUpdate, res, rules, false)
	if !got.Allowed || got.Layer != LayerException {
		t.Fatalf("%+v", got)
	}
}

func TestExceptionEndIsExclusive(t *testing.T) {
	end := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	res := &Resource{CompanyID: "co"}
	rules := []Rule{
		{
			ID: "ex", Domain: "geography", Action: ActionView, Effect: EffectAllow, Layer: LayerException,
			Scope: Scope{Kind: ScopeCompany, NodeID: "co"}, EndsAt: &end,
		},
		companyAllow(LayerDirect, ActionView),
	}
	got := Decide(end, activeSubject(), "geography", ActionView, res, rules, false)
	if !got.Allowed || got.Layer != LayerDirect {
		t.Fatalf("%+v", got)
	}
}

func TestInheritanceBreakWithoutDirectAllow(t *testing.T) {
	res := &Resource{CompanyID: "co"}
	rules := []Rule{companyAllow(LayerInherited, ActionPublish)}
	subject := activeSubject()
	got := Decide(time.Now(), subject, "geography", ActionPublish, res, rules, true)
	if got.Allowed || got.Code != "default_deny" {
		t.Fatalf("%+v", got)
	}
}

func TestViewDoesNotGrantExport(t *testing.T) {
	res := &Resource{CompanyID: "co"}
	rules := []Rule{companyAllow(LayerDirect, ActionView)}
	subject := activeSubject()
	view := Decide(time.Now(), subject, "geography", ActionView, res, rules, false)
	export := Decide(time.Now(), subject, "geography", ActionExport, res, rules, false)
	if !view.Allowed || export.Allowed || export.Code != "default_deny" {
		t.Fatalf("view %+v export %+v", view, export)
	}
}

func TestSeparationOfDuties(t *testing.T) {
	res := &Resource{CompanyID: "co", CreatedBy: "user"}
	rules := []Rule{companyAllow(LayerDirect, ActionApprove)}
	got := Decide(time.Now(), activeSubject(), "geography", ActionApprove, res, rules, false)
	if got.Allowed || got.Code != "separation_of_duties" {
		t.Fatalf("%+v", got)
	}
}

func TestBroaderDenyCoversChildBranch(t *testing.T) {
	rules := []Rule{
		companyAllow(LayerDirect, ActionUpdate),
		{
			ID: "branch-deny", Domain: "geography", Action: ActionUpdate, Effect: EffectDeny, Layer: LayerExplicitDeny,
			Scope: Scope{Kind: ScopeBranch, NodeID: "b1"}, Source: "branch-deny",
		},
	}
	inside := Decide(time.Now(), activeSubject(), "geography", ActionUpdate, &Resource{CompanyID: "co", BranchID: "b1", DepartmentID: "d1"}, rules, false)
	outside := Decide(time.Now(), activeSubject(), "geography", ActionUpdate, &Resource{CompanyID: "co", BranchID: "b2"}, rules, false)
	if inside.Allowed || inside.DenySource != "branch-deny" || !outside.Allowed {
		t.Fatalf("inside %+v outside %+v", inside, outside)
	}
}

func TestInactiveAccountAndCompanyMismatch(t *testing.T) {
	rules := []Rule{companyAllow(LayerDirect, ActionView)}
	locked := activeSubject()
	locked.Status = "locked"
	if got := Decide(time.Now(), locked, "geography", ActionView, &Resource{CompanyID: "co"}, rules, false); got.Code != "account_inactive" {
		t.Fatal(got)
	}
	if got := Decide(time.Now(), activeSubject(), "geography", ActionView, &Resource{CompanyID: "other"}, rules, false); got.Code != "company_mismatch" {
		t.Fatal(got)
	}
}

func TestStepUpRequiredForPublish(t *testing.T) {
	subject := activeSubject()
	subject.StepUp = false
	got := Decide(time.Now(), subject, "geography", ActionPublish, &Resource{CompanyID: "co", CreatedBy: "other"}, []Rule{companyAllow(LayerDirect, ActionPublish)}, false)
	if got.Code != "step_up_required" {
		t.Fatal(got)
	}
}

func TestSameLayerRestrictSubtracts(t *testing.T) {
	rules := []Rule{
		companyAllow(LayerDirect, ActionView),
		{
			Domain: "geography", Action: ActionView, Effect: EffectRestrict, Layer: LayerDirect,
			Scope: Scope{Kind: ScopeBranch, NodeID: "b1"},
		},
	}
	blocked := Decide(time.Now(), activeSubject(), "geography", ActionView, &Resource{CompanyID: "co", BranchID: "b1"}, rules, false)
	open := Decide(time.Now(), activeSubject(), "geography", ActionView, &Resource{CompanyID: "co", BranchID: "b2"}, rules, false)
	if blocked.Allowed || !open.Allowed {
		t.Fatalf("blocked %+v open %+v", blocked, open)
	}
}

func TestCollectCutsAncestorAllowsOnBreak(t *testing.T) {
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	nodes := []Node{{ID: "co"}, {ID: "team", ParentID: "co"}}
	memberships := []Membership{{NodeID: "team", Starts: now.Add(-time.Hour)}}
	rules := []Rule{{
		ID: "node-allow", Domain: "geography", Action: ActionView, Effect: EffectAllow, Layer: LayerInherited,
		Scope: Scope{Kind: ScopeCompany, NodeID: "co"}, AttachedTo: "node", GranteeID: "co",
	}}
	breaks := []Break{{NodeID: "team", Domain: "geography", Action: ActionView}}
	collected := Collect("user", nodes, memberships, rules, breaks, now)
	if len(collected) != 0 {
		t.Fatalf("ancestor allow should be cut: %+v", collected)
	}
	own := rules
	own = append(own, Rule{
		Domain: "geography", Action: ActionView, Effect: EffectAllow, Layer: LayerInherited,
		Scope: Scope{Kind: ScopeTeam, NodeID: "team"}, AttachedTo: "node", GranteeID: "team",
	})
	collected = Collect("user", nodes, memberships, own, breaks, now)
	if len(collected) != 1 || collected[0].GranteeID != "team" {
		t.Fatalf("%+v", collected)
	}
}

func TestEndedMembershipDoesNotGrant(t *testing.T) {
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	end := now
	nodes := []Node{{ID: "co"}}
	memberships := []Membership{{NodeID: "co", Starts: now.Add(-time.Hour), Ends: &end}}
	rules := []Rule{{
		Domain: "organization", Action: ActionView, Effect: EffectAllow,
		Scope: Scope{Kind: ScopeCompany, NodeID: "co"}, AttachedTo: "node", GranteeID: "co",
	}}
	if got := Collect("user", nodes, memberships, rules, nil, now); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestExceptionConflict(t *testing.T) {
	now := time.Now()
	end := now.Add(time.Hour)
	rules := []Rule{
		{Domain: "geography", Action: ActionUpdate, Layer: LayerException, Effect: EffectAllow, GranteeID: "user", Scope: Scope{Kind: ScopeBranch, NodeID: "b"}, EndsAt: &end},
		{Domain: "geography", Action: ActionUpdate, Layer: LayerException, Effect: EffectRestrict, GranteeID: "user", Scope: Scope{Kind: ScopeBranch, NodeID: "b"}, EndsAt: &end},
	}
	if !ExceptionConflict(rules, now) {
		t.Fatal("expected conflict")
	}
}

func TestWouldCycle(t *testing.T) {
	parent := map[string]string{"team": "dept", "dept": "branch", "branch": "co"}
	if !WouldCycle(parent, "co", "team") {
		t.Fatal("moving company under its descendant must cycle")
	}
	if WouldCycle(parent, "dept", "co") {
		t.Fatal("moving a node under an ancestor is not a cycle")
	}
}

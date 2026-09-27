package permissions

import "time"

type Action string

const (
	ActionView              Action = "view"
	ActionCreate            Action = "create"
	ActionUpdate            Action = "update"
	ActionArchive           Action = "archive"
	ActionDeleteDraft       Action = "delete_draft"
	ActionApprove           Action = "approve"
	ActionPublish           Action = "publish"
	ActionExport            Action = "export"
	ActionReveal            Action = "reveal"
	ActionSecurity          Action = "security"
	ActionPayment           Action = "payment"
	ActionMerge             Action = "merge"
	ActionChangeBeneficiary Action = "change_beneficiary"
)

type ScopeKind string

const (
	ScopeOwn        ScopeKind = "own"
	ScopeAssigned   ScopeKind = "assigned"
	ScopeTeam       ScopeKind = "team"
	ScopeDepartment ScopeKind = "department"
	ScopeBranch     ScopeKind = "branch"
	ScopeCompany    ScopeKind = "company"
)

type Layer string

const (
	LayerExplicitDeny Layer = "explicit_deny"
	LayerException    Layer = "exception"
	LayerDirect       Layer = "direct"
	LayerInherited    Layer = "inherited"
	LayerDefaultDeny  Layer = "default_deny"
	LayerConstraint   Layer = "constraint"
)

type Effect string

const (
	EffectAllow    Effect = "allow"
	EffectDeny     Effect = "deny"
	EffectRestrict Effect = "restrict"
)

type Scope struct {
	Kind   ScopeKind
	NodeID string
}

type Rule struct {
	ID         string
	Domain     string
	Action     Action
	Effect     Effect
	Layer      Layer
	Scope      Scope
	StartsAt   *time.Time
	EndsAt     *time.Time
	Source     string
	GranteeID  string
	AttachedTo string
}

type Subject struct {
	UserID    string
	CompanyID string
	Status    string
	StepUp    bool
}

type Resource struct {
	CompanyID    string
	BranchID     string
	DepartmentID string
	TeamID       string
	OwnerID      string
	AssigneeID   string
	CreatedBy    string
}

type Decision struct {
	Allowed    bool
	Layer      Layer
	Code       string
	DenySource string
}

func Decide(now time.Time, subject Subject, domain string, action Action, resource *Resource, rules []Rule, inheritanceBroken bool) Decision {
	if subject.Status != "active" {
		return Decision{Layer: LayerConstraint, Code: "account_inactive"}
	}
	if resource != nil && resource.CompanyID != "" && resource.CompanyID != subject.CompanyID {
		return Decision{Layer: LayerConstraint, Code: "company_mismatch"}
	}
	if needsStepUp(action) && !subject.StepUp {
		return Decision{Layer: LayerConstraint, Code: "step_up_required"}
	}
	if resource != nil && separatesDuties(action) && resource.CreatedBy != "" && resource.CreatedBy == subject.UserID {
		return Decision{Layer: LayerConstraint, Code: "separation_of_duties"}
	}

	var matched []Rule
	for _, rule := range rules {
		if rule.Domain != domain || rule.Action != action || !active(rule, now) {
			continue
		}
		matched = append(matched, rule)
	}

	var denies []Rule
	var exceptions []Rule
	var directs []Rule
	var inherited []Rule
	for _, rule := range matched {
		if rule.Effect == EffectDeny || rule.Layer == LayerExplicitDeny {
			denies = append(denies, rule)
			continue
		}
		switch rule.Layer {
		case LayerException:
			exceptions = append(exceptions, rule)
		case LayerDirect:
			directs = append(directs, rule)
		case LayerInherited:
			inherited = append(inherited, rule)
		}
	}

	layer := LayerDefaultDeny
	var pool []Rule
	switch {
	case hasAllow(exceptions):
		layer = LayerException
		pool = exceptions
	case hasAllow(directs):
		layer = LayerDirect
		pool = directs
	case !inheritanceBroken && hasAllow(inherited):
		layer = LayerInherited
		pool = inherited
	default:
		return Decision{Layer: LayerDefaultDeny, Code: "default_deny"}
	}

	allows := filterEffect(pool, EffectAllow)
	restricts := filterEffect(pool, EffectRestrict)
	if resource == nil {
		if len(allows) == 0 {
			return Decision{Layer: layer, Code: "outside_scope"}
		}
		return Decision{Allowed: true, Layer: layer, Code: "allowed"}
	}
	allowed := false
	for _, rule := range allows {
		if rule.Scope.Matches(subject, *resource) && !coveredBy(restricts, subject, *resource) && !coveredBy(denies, subject, *resource) {
			allowed = true
			break
		}
	}
	if allowed {
		return Decision{Allowed: true, Layer: layer, Code: "allowed"}
	}
	if src := matchingSource(denies, subject, *resource); src != "" {
		return Decision{Layer: LayerExplicitDeny, Code: "explicit_deny", DenySource: src}
	}
	return Decision{Layer: layer, Code: "outside_scope"}
}

func (s Scope) Matches(subject Subject, resource Resource) bool {
	switch s.Kind {
	case ScopeCompany:
		return resource.CompanyID != "" && resource.CompanyID == subject.CompanyID && (s.NodeID == "" || s.NodeID == resource.CompanyID)
	case ScopeBranch:
		return s.NodeID != "" && resource.BranchID == s.NodeID
	case ScopeDepartment:
		return s.NodeID != "" && resource.DepartmentID == s.NodeID
	case ScopeTeam:
		return s.NodeID != "" && resource.TeamID == s.NodeID
	case ScopeAssigned:
		return resource.AssigneeID != "" && resource.AssigneeID == subject.UserID
	case ScopeOwn:
		return resource.OwnerID != "" && resource.OwnerID == subject.UserID
	default:
		return false
	}
}

func ExceptionConflict(rules []Rule, now time.Time) bool {
	activeOnes := make([]Rule, 0)
	for _, rule := range rules {
		if rule.Layer == LayerException && active(rule, now) {
			activeOnes = append(activeOnes, rule)
		}
	}
	for i := 0; i < len(activeOnes); i++ {
		for j := i + 1; j < len(activeOnes); j++ {
			a, b := activeOnes[i], activeOnes[j]
			if a.Domain != b.Domain || a.Action != b.Action || a.GranteeID != b.GranteeID {
				continue
			}
			if !overlaps(a, b) {
				continue
			}
			if a.Effect == b.Effect {
				continue
			}
			if a.Scope.Kind == b.Scope.Kind && a.Scope.NodeID == b.Scope.NodeID {
				return true
			}
		}
	}
	return false
}

func needsStepUp(action Action) bool {
	switch action {
	case ActionPublish, ActionApprove, ActionExport, ActionReveal, ActionSecurity, ActionPayment, ActionMerge, ActionChangeBeneficiary:
		return true
	default:
		return false
	}
}

func separatesDuties(action Action) bool {
	return action == ActionApprove || action == ActionPublish
}

func active(rule Rule, now time.Time) bool {
	if rule.StartsAt != nil && now.Before(*rule.StartsAt) {
		return false
	}
	if rule.EndsAt != nil && !now.Before(*rule.EndsAt) {
		return false
	}
	return true
}

func overlaps(a, b Rule) bool {
	startA, endA := window(a)
	startB, endB := window(b)
	return startA.Before(endB) && startB.Before(endA)
}

func window(rule Rule) (time.Time, time.Time) {
	start := time.Time{}
	end := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	if rule.StartsAt != nil {
		start = *rule.StartsAt
	}
	if rule.EndsAt != nil {
		end = *rule.EndsAt
	}
	return start, end
}

func hasAllow(rules []Rule) bool {
	for _, rule := range rules {
		if rule.Effect == EffectAllow {
			return true
		}
	}
	return false
}

func filterEffect(rules []Rule, effect Effect) []Rule {
	out := make([]Rule, 0)
	for _, rule := range rules {
		if rule.Effect == effect {
			out = append(out, rule)
		}
	}
	return out
}

func coveredBy(rules []Rule, subject Subject, resource Resource) bool {
	for _, rule := range rules {
		if rule.Scope.Matches(subject, resource) {
			return true
		}
	}
	return false
}

func matchingSource(rules []Rule, subject Subject, resource Resource) string {
	for _, rule := range rules {
		if rule.Scope.Matches(subject, resource) {
			if rule.Source != "" {
				return rule.Source
			}
			return rule.ID
		}
	}
	return ""
}

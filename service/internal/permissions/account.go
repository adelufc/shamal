package permissions

import "time"

type RuleView struct {
	ID          string     `json:"id"`
	Domain      string     `json:"domain"`
	Action      string     `json:"action"`
	Effect      string     `json:"effect"`
	Layer       string     `json:"layer"`
	ScopeKind   string     `json:"scope_kind"`
	ScopeNodeID string     `json:"scope_node_id"`
	Source      string     `json:"source"`
	GranteeID   string     `json:"grantee_id"`
	StartsAt    *time.Time `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
	Temporary   bool       `json:"temporary"`
}

type BreakView struct {
	NodeID string `json:"node_id"`
	Domain string `json:"domain"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type Account struct {
	Direct    []RuleView  `json:"direct"`
	Inherited []RuleView  `json:"inherited"`
	Temporary []RuleView  `json:"temporary"`
	Deny      []RuleView  `json:"deny"`
	Breaks    []BreakView `json:"breaks"`
}

func ViewRule(rule Rule) RuleView {
	return RuleView{
		ID: rule.ID, Domain: rule.Domain, Action: string(rule.Action), Effect: string(rule.Effect), Layer: string(rule.Layer),
		ScopeKind: string(rule.Scope.Kind), ScopeNodeID: rule.Scope.NodeID, Source: rule.Source, GranteeID: rule.GranteeID,
		StartsAt: rule.StartsAt, EndsAt: rule.EndsAt, Temporary: rule.StartsAt != nil || rule.EndsAt != nil,
	}
}

func Present(rules []Rule, breaks []BreakView) Account {
	account := Account{
		Direct: make([]RuleView, 0), Inherited: make([]RuleView, 0), Temporary: make([]RuleView, 0),
		Deny: make([]RuleView, 0), Breaks: breaks,
	}
	if account.Breaks == nil {
		account.Breaks = make([]BreakView, 0)
	}
	for _, rule := range rules {
		view := ViewRule(rule)
		if view.Effect == string(EffectDeny) || view.Layer == string(LayerExplicitDeny) {
			account.Deny = append(account.Deny, view)
		} else if view.Layer == string(LayerInherited) {
			account.Inherited = append(account.Inherited, view)
		} else {
			account.Direct = append(account.Direct, view)
		}
		if view.Temporary {
			account.Temporary = append(account.Temporary, view)
		}
	}
	return account
}

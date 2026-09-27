package permissions

import "time"

type Node struct {
	ID       string
	ParentID string
}

type Membership struct {
	NodeID  string
	Starts  time.Time
	Ends    *time.Time
}

type Break struct {
	NodeID string
	Domain string
	Action Action
}

func MembershipActive(m Membership, now time.Time) bool {
	if now.Before(m.Starts) {
		return false
	}
	if m.Ends != nil && !now.Before(*m.Ends) {
		return false
	}
	return true
}

func Collect(userID string, nodes []Node, memberships []Membership, rules []Rule, breaks []Break, now time.Time) []Rule {
	parent := map[string]string{}
	for _, node := range nodes {
		parent[node.ID] = node.ParentID
	}
	out := make([]Rule, 0)
	for _, rule := range rules {
		if rule.AttachedTo == "user" && rule.GranteeID == userID {
			out = append(out, rule)
		}
	}
	for _, membership := range memberships {
		if !MembershipActive(membership, now) {
			continue
		}
		path := ancestors(membership.NodeID, parent)
		onPath := map[string]int{}
		for i, id := range path {
			onPath[id] = i
		}
		for _, rule := range rules {
			if rule.AttachedTo != "node" {
				continue
			}
			grantAt, ok := onPath[rule.GranteeID]
			if !ok {
				continue
			}
			if cutByBreak(path, onPath, grantAt, rule, breaks) {
				continue
			}
			copied := rule
			if rule.Effect == EffectDeny {
				copied.Layer = LayerExplicitDeny
			} else if copied.Layer == "" {
				copied.Layer = LayerInherited
			} else if copied.Layer != LayerExplicitDeny {
				copied.Layer = LayerInherited
			}
			out = append(out, copied)
		}
	}
	return out
}

func ancestors(id string, parent map[string]string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		out = append(out, id)
		id = parent[id]
	}
	return out
}

func cutByBreak(path []string, index map[string]int, grantAt int, rule Rule, breaks []Break) bool {
	for _, item := range breaks {
		if item.Action != rule.Action || (item.Domain != "" && item.Domain != rule.Domain) {
			continue
		}
		at, ok := index[item.NodeID]
		if !ok {
			continue
		}
		if at < grantAt {
			return true
		}
	}
	return false
}

func WouldCycle(parent map[string]string, id, newParent string) bool {
	if id == "" || newParent == "" {
		return false
	}
	if id == newParent {
		return true
	}
	seen := map[string]bool{}
	for newParent != "" && !seen[newParent] {
		if newParent == id {
			return true
		}
		seen[newParent] = true
		newParent = parent[newParent]
	}
	return false
}

package organization

func CloseBlocked(activeChildren, activeMemberships, activeRecords int) bool {
	return activeChildren > 0 || activeMemberships > 0 || activeRecords > 0
}

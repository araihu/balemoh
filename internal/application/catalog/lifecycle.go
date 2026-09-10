package catalog

// LifecycleTargets resolves the current group and validates actions against
// server truth. Storage calls this inside its write transaction.
func LifecycleTargets(observations []Candidate, id, action string) ([]string, error) {
	if action != "hide" && action != "show" && action != "purge" {
		return nil, ErrConflict
	}
	for _, group := range groupCandidates(observations) {
		if group.ID != id {
			continue
		}
		if action == "hide" && group.Missing {
			return nil, ErrConflict
		}
		if action == "purge" && !group.Missing {
			return nil, ErrConflict
		}
		members := map[ResourceRef]bool{group.Resource: true}
		for _, r := range group.Resources {
			members[r.Resource] = true
		}
		var ids []string
		for _, c := range observations {
			if c.Source != group.Source || !members[c.Resource] {
				continue
			}
			if action == "purge" && !c.Missing {
				continue
			}
			ids = append(ids, c.ID)
		}
		return ids, nil
	}
	return nil, ErrNotFound
}

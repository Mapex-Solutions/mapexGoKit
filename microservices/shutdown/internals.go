package shutdown

// groupByPriority groups hooks by their priority value.
// Input must be sorted by priority ascending.
func groupByPriority(hooks []shutdownHook) [][]shutdownHook {
	if len(hooks) == 0 {
		return nil
	}

	var groups [][]shutdownHook
	current := []shutdownHook{hooks[0]}

	for i := 1; i < len(hooks); i++ {
		if hooks[i].Priority != hooks[i-1].Priority {
			groups = append(groups, current)
			current = []shutdownHook{hooks[i]}
		} else {
			current = append(current, hooks[i])
		}
	}
	groups = append(groups, current)

	return groups
}

package executor

import "github.com/eugenetaranov/bolt/internal/playbook"

// effectiveTags computes the union of a task's own tags and all inherited tags.
func effectiveTags(task *playbook.Task, playTags, blockTags []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, tags := range [][]string{playTags, blockTags, task.Tags} {
		for _, tag := range tags {
			if !seen[tag] {
				seen[tag] = true
				result = append(result, tag)
			}
		}
	}
	return result
}

// shouldRunTask determines whether a task should execute based on tag filters.
//
// Rules:
//   - If no tags/skipTags filters are active, all tasks run except those tagged "never".
//   - If tags filter is active, only tasks with at least one matching effective tag run.
//   - Tasks tagged "always" run even when tags filter is active and other tags don't match.
//   - Tasks tagged "never" are skipped unless one of their other tags is in the tags filter.
//   - skipTags takes precedence: tasks matching any skip-tag are skipped (including "always").
func shouldRunTask(eTags []string, tags []string, skipTags []string) bool {
	tagSet := toSet(eTags)
	filterSet := toSet(tags)
	skipSet := toSet(skipTags)

	hasAlways := tagSet["always"]
	hasNever := tagSet["never"]

	// Check skip-tags first (highest precedence)
	if len(skipSet) > 0 {
		for tag := range tagSet {
			if skipSet[tag] {
				return false
			}
		}
	}

	// No --tags filter active
	if len(filterSet) == 0 {
		// "never" tasks are skipped by default
		if hasNever {
			return false
		}
		return true
	}

	// --tags filter is active

	// "always" tasks run regardless of filter
	if hasAlways {
		return true
	}

	// "never" tasks: run only if one of their tags is explicitly in the filter
	// (We already handled skip-tags above, so just check inclusion.)

	// Check if any effective tag matches the filter
	for tag := range tagSet {
		if filterSet[tag] {
			return true
		}
	}

	return false
}

func toSet(tags []string) map[string]bool {
	if len(tags) == 0 {
		return nil
	}
	s := make(map[string]bool, len(tags))
	for _, t := range tags {
		s[t] = true
	}
	return s
}

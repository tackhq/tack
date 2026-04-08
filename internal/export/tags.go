package export

import "github.com/tackhq/tack/internal/playbook"

// effectiveTags computes the union of a task's own tags and all inherited tags.
// Mirrors executor.effectiveTags.
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
// Mirrors executor.shouldRunTask.
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
		return !hasNever
	}

	// --tags filter is active
	if hasAlways {
		return true
	}

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

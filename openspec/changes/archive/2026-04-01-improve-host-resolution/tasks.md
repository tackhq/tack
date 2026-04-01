## 1. Inventory: AllHosts method

- [x] 1.1 Add `AllHosts() []string` method to `*Inventory` that expands every group, merges top-level hosts, and deduplicates
- [x] 1.2 Add unit tests for `AllHosts()` — multiple groups, overlapping hosts, empty inventory, groups with SSM instances

## 2. Executor: --hosts all expansion

- [x] 2.1 In `runPlay` inventory expansion loop, detect the `"all"` keyword and call `inventory.AllHosts()` to replace it with the full host list
- [x] 2.2 Add error when `"all"` is used without an inventory: `--hosts all requires an inventory file (-i flag)`
- [x] 2.3 Add unit tests for "all" expansion in executor

## 3. Error message improvements

- [x] 3.1 Update the generic missing-hosts error (executor.go:333) to mention `--hosts`, playbook `hosts:` field, and `-c` flag
- [x] 3.2 Add distinct error after SSM tag resolution returns zero instances — include the tags in the message
- [x] 3.3 Add unit tests for both error paths

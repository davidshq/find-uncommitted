## 1. Matrix cell formatting

- [x] 1.1 Add a shared compact cell formatter (reuse/adapt `formatCheckMachineCell`) for matrix columns: clean / dirty / ↑N / behind / diverged / empty / error / stale
- [x] 1.2 Define rules for when branch appears in a cell (mismatch or material glance aid)
- [x] 1.3 Handle multiple clones of one project on one machine: worst-status collapse for the cell

## 2. Matrix display

- [x] 2.1 Build machine column set from aggregate rows (local first, mark with `*`)
- [x] 2.2 Implement `displayProjectMachineMatrix` from `GroupRowsByProject` + columns
- [x] 2.3 Wire default tree-scan path in `main.go` to print matrix (not Attention + path inventory)
- [x] 2.4 Apply `--dirty-only` filtering to matrix rows using existing situation predicates; keep load-error visibility

## 3. Opt-in inventory

- [x] 3.1 Add `--inventory` flag (and optionally map `--verbose` to the same path table)
- [x] 3.2 When inventory is set, print path-centric `displayAggregateTable` (and Attention nudges if keeping “old full view”)
- [x] 3.3 Ensure default output never stacks Attention + matrix + full inventory

## 4. Tests and docs

- [x] 4.1 Add unit tests for matrix layout / cell formatting / dirty-only project filtering
- [x] 4.2 Update any stdout tests that assume Attention-then-Full-inventory as default
- [x] 4.3 Update README output examples and flag help; note BREAKING default human output
- [x] 4.4 Confirm `check`, CSV export, agent, and extension paths are unchanged

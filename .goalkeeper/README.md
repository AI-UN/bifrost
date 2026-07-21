# Goal Keeper Workspace

This directory tracks the active long-running objective for this repository.

Status flow:
- `clarifying`: required goal inputs are still missing
- `goal-ready`: the brief is stable enough to survive execution
- `plan-draft`: a phased execution plan exists but is not yet approved
- `plan-accepted`: the user approved the plan and `/goal` may begin execution

Primary files:
- `active-goal.md`: source of truth for the currently active goal
- `goals/<slug>/brief.md`: objective, success criteria, constraints, non-goals, risks
- `goals/<slug>/plan.md`: phased execution plan
- `goals/<slug>/progress.md`: atomic tasks and dependency tracking
- `goals/<slug>/memory.md`: durable evidence and decisions
- `goals/<slug>/review.md`: review gates and acceptance notes

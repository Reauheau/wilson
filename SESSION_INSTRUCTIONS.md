# Session Instructions

## Communication Style
- Be pragmatic, not enthusiastic
- No excessive emoji or "celebration" language
- Skip verbose explanations unless asked
- Only create documentation files when explicitly requested or in plan mode

## Todo Management
- Use TodoWrite to track tasks during work
- When completing todos, move valuable items to DONE.md
- Delete trivial todos (don't clutter DONE.md with minor fixes)

## Key Files
Read at session start if relevant to task:
- `TODO.md` - Current priorities
- `DONE.md` - Completed work history
- `ENDGAME.md` - Multi-agent system vision

## Development Standards

### Testing
- Run tests after code changes
- Document failures and fixes
- Phase tests: `test_[phase_name].go`

### Tools
Must implement: `Metadata()`, `Validate()`, `Execute()`

Categories: filesystem, web, context, agent, system

### Agents
Pattern: BaseAgent → Purpose-specific LLM → Tool restrictions → System prompt

Communication: Use `LeaveNote()` and `StoreArtifact()`

### Critical
- Code Agent needs write_file and modify_file tools
- Validate paths are relative (security)
- Context DB: contexts, artifacts, agent_notes, tasks, task_reviews

## Phase Status
Track ENDGAME phases in TODO.md before starting work.

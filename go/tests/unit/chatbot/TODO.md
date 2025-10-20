# Chatbot Tests TODO

## Completed ✅
- ✅ Intent classification tests (18 test cases, 100% pass)
- ✅ Prompt generation tests (chat, system, caching)
- ✅ Prompt size comparison tests
- ✅ Performance benchmarks

## Future Work
### Chat Handler Tests (Phase 1-2 completion)
**Status:** Deferred - requires mocking infrastructure

Would test:
- Chat handler flow (HandleChat)
- Intent-based prompt selection
- History management integration
- Tool execution flow
- Error handling

**Blockers:**
- Needs Ollama mock/stub
- Needs session.History mock
- Needs registry.Executor mock

**Recommendation:** Implement after Phase 3 when we have better understanding of async flows and can create reusable mocks.

### Integration Tests (Phase 1-2)
**Location:** `tests/integration/chatbot/`

Would test:
- End-to-end chat flow
- Simple chat → minimal prompt → response
- Tool request → full prompt → tool execution → response
- Delegation request detection

**Status:** Can be added incrementally as Phase 3-7 progress

---

**Current Coverage:**
- Unit tests: 7 tests (intent classification, prompts, caching)
- Integration tests: 0 (future work)
- Benchmarks: 4 (prompt generation performance)

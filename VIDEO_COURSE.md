# Video Course Scripts

## Episode 1: Introduction to LLMOrchestrator

**Duration**: 8 minutes

1. What is LLMOrchestrator? (1 min)
2. Problem: managing multiple CLI agents (2 min)
3. Architecture overview with Mermaid diagram (2 min)
4. Quick demo: starting agents and sending prompts (3 min)

## Episode 2: Agent Interface and Adapters

**Duration**: 10 minutes

1. The Agent interface contract (2 min)
2. BaseAdapter: shared process management (3 min)
3. Walk through ClaudeCodeAgent implementation (2 min)
4. Creating a custom adapter (3 min)

## Episode 3: Agent Pool and Circuit Breaker

**Duration**: 10 minutes

1. Thread-safe AgentPool design (2 min)
2. Capability-based matching with Acquire() (2 min)
3. sync.Mutex + sync.Cond pattern (2 min)
4. Circuit breaker: closed/open/half-open states (2 min)
5. HealthMonitor integration (2 min)

## Episode 4: Communication Protocol

**Duration**: 10 minutes

1. JSON-lines pipe protocol (2 min)
2. PipeTransport: send/receive with context cancellation (3 min)
3. FileTransport: inbox/outbox/shared directories (3 min)
4. Security: path traversal protection (2 min)

## Episode 5: Testing Strategy

**Duration**: 12 minutes

1. Test types: unit, integration, stress, security, fuzz (2 min)
2. Testing the AgentPool with mock agents (3 min)
3. Fuzz testing the ResponseParser (3 min)
4. Stress testing concurrent pool operations (2 min)
5. Automation tests: project structure validation (2 min)

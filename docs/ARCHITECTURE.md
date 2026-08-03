# AffectWeave Architecture

## 1. Context Pipeline

AffectWeave separates persistent state, context selection and host rendering:

```text
Core State
    ↓
TurnPreparation
    ↓
ContextCompiler
    ↓
ContextPackage
    ├── TextPromptRenderer
    ├── JSONRenderer
    └── MessageBlockRenderer
    ↓
EnhancementPacket
    ↓
Host AI
```

The complete feedback loop is:

```text
prepare_turn
→ host AI
→ commit_output
→ record_delivery
→ record_outcome
```

## 2. Package boundaries

### `affectweave.core`

Owns persistent state and deterministic rules:

- frozen Persona;
- medium-term state;
- causal emotion episodes;
- per-entity relationship state;
- active threads;
- evidence-backed episodic memories;
- behavior records;
- clamping, versions and idempotency;
- JSON and SQLite persistence;
- L0/L1/L2/L3 closure.

### `affectweave.compiler.context`

Owns context selection and budgeting:

- current event preservation;
- character projection;
- active emotion and relationship digest;
- ActiveThread preservation;
- memory and recent-dialogue selection;
- optional-section trimming;
- `ContextBudgetReport` and `ContextTrace`.

It produces a host-neutral `ContextPackage`.

### `affectweave.compiler.renderers`

Owns output formatting:

- `TextPromptRenderer`;
- `JSONRenderer`;
- `MessageBlockRenderer`.

Renderers receive one immutable context package and do not modify Core state.

### `affectweave.compiler.packet`

Owns the exchange contract:

- `ContextPackage`;
- `EnhancementPacket`;
- output JSON Schema;
- serialization and compatibility loading.

### `affectweave.compiler.validator`

Validates host AI output before Core updates are accepted.

### `affectweave.integration`

Coordinates Core, ContextCompiler, Renderer and Validator through the public lifecycle.

## 3. Dependency direction

```text
core        -> Python standard library
context     -> core + packet
renderers   -> context package
validator   -> core data types
integration -> core + compiler
cli         -> integration
host        -> integration or cli
```

## 4. Budgeting

The compiler uses a provider-neutral token estimate. It preserves the current event and base ActiveThread fields, then removes optional content in this order:

1. voice notes and contradictions;
2. lowest-priority memories;
3. oldest recent dialogue above the minimum;
4. optional preferences and low-priority values/traits;
5. optional ActiveThread details;
6. verbose rule and task wording.

The result is recorded in `ContextBudgetReport`:

```text
maximum_tokens
estimated_tokens
overflow
included_sections
removed_sections
removal_reasons
```

## 5. EnhancementPacket

The packet contains:

```text
ContextPackage
Default RenderedContext
Output JSON Schema
Turn and behavior identifiers
```

Convenience methods:

```python
packet.to_prompt()
packet.to_context_json()
packet.to_messages()
packet.to_json()          # full packet serialization
```

## 6. Compatibility

`PromptCompiler` and `PromptConfig` remain as compatibility wrappers around `ContextCompiler` and `ContextConfig`. Existing hosts can continue using `packet.prompt_fragment` and full packet JSON serialization.

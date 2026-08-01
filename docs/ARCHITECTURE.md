# AffectWeave Architecture

## 1. Positioning

AffectWeave is a persistent character-state and prompt-enhancement layer for AI hosts.
It maintains character continuity outside the model context, compiles the current
state into a compact packet, and closes the state loop after the host reports what
actually happened.

```text
Host application
    │
    ├── asks AffectWeave to prepare the current character context
    ├── invokes its own AI/model
    ├── returns the model JSON to AffectWeave for validation
    ├── displays or sends the reply
    └── reports delivery and later outcome evidence
```

The stable design remains:

```text
Persona + Affect/Relationship + Verified Behavior Result
```

## 2. Package boundaries

### `affectweave.core`

Owns persistent state and deterministic rules:

- frozen Persona;
- medium-term state;
- causal emotion episodes;
- per-entity relationship state;
- unresolved active threads;
- episodic memories with evidence levels;
- behavior records;
- numeric clamping;
- optimistic state versions;
- idempotency;
- JSON/SQLite persistence;
- L0/L1/L2/L3 behavior closure.

Core never imports Compiler and never calls a model.

### `affectweave.compiler`

Owns the exchange contract between AffectWeave and an arbitrary host AI:

- `EnhancementPacket`;
- prompt-fragment compilation;
- token-budget trimming;
- provider-neutral JSON Schema;
- parsing and validation of host AI output;
- protected-field rejection;
- speech-budget validation.

Compiler imports Core data types but does not mutate the repository directly.

### `affectweave.integration`

Provides the small public facade a host normally uses:

```text
prepare_turn
commit_output
record_delivery
record_outcome
```

It coordinates Core and Compiler. It does not invoke a model.

### `affectweave.cli`

Exposes the same passive lifecycle for shell-based hosts and testing:

```text
affectweave init
affectweave prepare
affectweave commit
affectweave delivery
affectweave outcome
affectweave state
```

## 3. Dependency direction

```text
core        -> Python standard library
compiler    -> core + Python standard library
integration -> core + compiler
cli         -> integration
host        -> integration or cli
```

Forbidden dependencies:

```text
core -> model provider
core -> HTTP client
compiler -> OpenAI / Anthropic / DeepSeek
integration -> API key management
AffectWeave -> host chat runtime
```

## 4. Turn lifecycle

### Step 1: Prepare

The host sends the current user event to AffectWeave.

Core:

- stores the raw event;
- creates a pending behavior record;
- retrieves relevant memories;
- produces a compact state snapshot.

Compiler turns that into an `EnhancementPacket`.

### Step 2: Host model call

The host decides how to use the packet:

- append `prompt_fragment` to a system/developer message;
- use `output_schema` as native structured-output schema;
- or include `packet.to_prompt(include_schema=True)` in a plain prompt.

AffectWeave has no knowledge of the provider or transport.

### Step 3: Commit model output

The host returns one JSON object.

Compiler validates:

- structure;
- protected fields;
- allowed goals and acts;
- maximum list counts;
- reply sentence/token budget;
- question prohibition.

Core then clamps all accepted state deltas and stores the generated reply as
`PENDING / L0`.

### Step 4: Record actual delivery

Only after the host really displays or sends the reply does it call
`record_delivery()`.

A successful delivery produces `DELIVERED / L1`. A failed delivery remains
`FAILED / L0`.

### Step 5: Record observed outcome

A later observed or verified result can be recorded as L2 or L3. Only then can
pending memory candidates be committed.

## 5. Why the packet is versioned

Hosts usually cannot erase older context already present in a conversation.
Each packet therefore includes:

- `schema_version`;
- `state_version`;
- `turn_id`;
- `behavior_id`.

The prompt explicitly says that the current packet is the authoritative state
for the current turn. A host should inject only one current packet per call.

## 6. State safety

The model cannot write:

- Persona;
- safety rules;
- permissions;
- tools;
- system instructions;
- absolute state;
- API credentials or provider configuration.

The writable contract contains only bounded proposals. Core clamps them again,
so prompt compliance is not the final trust boundary.

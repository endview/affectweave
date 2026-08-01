# AffectWeave Host Integration

## 1. Integration contract

AffectWeave is designed to sit beside an existing AI host.

The host must implement four moments:

```text
on_user_message     -> AffectWeave prepare_turn
before_model_call   -> inject EnhancementPacket
on_model_output     -> AffectWeave commit_output
on_actual_delivery  -> AffectWeave record_delivery
on_observed_result  -> AffectWeave record_outcome
```

The host keeps control of:

- model/provider selection;
- API keys;
- system prompt construction;
- tools;
- streaming;
- message splitting;
- retries;
- UI and delivery.

## 2. Python API

### Create a character

```python
from datetime import datetime, timezone

from affectweave import AffectWeave
from affectweave.core import (
    SQLiteRepository,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)

now = datetime.now(timezone.utc)
repository = SQLiteRepository("runtime/affectweave.sqlite3")
affectweave = AffectWeave(repository)

state = affectweave.create_character(
    "maya",
    persona=make_maya_bennett_persona(),
    medium=make_maya_bennett_medium(now),
    now=now,
)
```

### Prepare the current turn

```python
from affectweave.core import InputEvent

packet = affectweave.prepare_turn(
    "maya",
    InputEvent(
        actor_id="mighon",
        message="Hello, Maya.",
        occurred_at=now,
    ),
    idempotency_key="session-1:turn-1:prepare",
    expected_state_version=state.version,
)
```

The host receives:

```python
packet.prompt_fragment
packet.output_schema
packet.behavior_id
packet.state_version
packet.estimated_tokens
```

### Invoke the host's existing model

Pseudocode:

```python
raw_output = existing_ai.generate_json(
    system_context=[base_prompt, packet.prompt_fragment],
    output_schema=packet.output_schema,
    user_message=current_user_message,
)
```

AffectWeave does not implement `existing_ai.generate_json`.

### Validate and commit the model output

```python
committed = affectweave.commit_output(
    "maya",
    packet,
    raw_output,
    idempotency_key="session-1:turn-1:commit",
)

reply = committed.behavior.generated_text
```

At this point the behavior is still `PENDING / L0`.

### Report actual delivery

```python
delivered = affectweave.record_delivery(
    "maya",
    packet.behavior_id,
    reply,
    idempotency_key="session-1:turn-1:delivery",
)
```

Successful delivery becomes `DELIVERED / L1`.

### Report a later result

```python
from affectweave.core import EvidenceLevel, OutcomeEvent

affectweave.record_outcome(
    "maya",
    packet.behavior_id,
    OutcomeEvent(
        outcome_id="session-1:turn-1:user-response",
        evidence_level=EvidenceLevel.L2,
        occurred_at=next_message_time,
        summary="The user continued normally.",
    ),
    idempotency_key="session-1:turn-1:outcome",
)
```

## 3. Prompt injection forms

### Host supports structured output

Use:

```python
prompt = packet.prompt_fragment
schema = packet.output_schema
```

This is the preferred form because the full JSON Schema does not need to be
repeated inside natural-language instructions.

### Host only supports text prompts

Use:

```python
prompt = packet.to_prompt(include_schema=True)
```

Then parse the returned JSON and pass it to `commit_output()`.

### Host uses an internal message object

Wrap `prompt_fragment` in whichever instruction role the host supports. AffectWeave
does not prescribe OpenAI, Anthropic, Codex, Claude Code, or any other message
format.

## 4. CLI integration

Initialize:

```bash
affectweave --db runtime/affectweave.sqlite3 init --character maya
```

Prepare a packet:

```bash
affectweave --db runtime/affectweave.sqlite3 prepare \
  --character maya \
  --actor mighon \
  --message "Hello, Maya" \
  --idempotency-key session-1-turn-1 \
  --packet-file /tmp/affectweave-packet.json
```

After the host model writes JSON to `/tmp/model-output.json`:

```bash
affectweave --db runtime/affectweave.sqlite3 commit \
  --character maya \
  --packet-file /tmp/affectweave-packet.json \
  --output-file /tmp/model-output.json \
  --idempotency-key session-1-turn-1-commit
```

Record the actual delivered reply:

```bash
affectweave --db runtime/affectweave.sqlite3 delivery \
  --character maya \
  --behavior <behavior-id> \
  --text-file /tmp/reply.txt \
  --idempotency-key session-1-turn-1-delivery
```

Inspect state:

```bash
affectweave --db runtime/affectweave.sqlite3 state --character maya
```

## 5. Idempotency and versions

Use stable idempotency keys based on host session and turn identifiers.

Recommended pattern:

```text
<session-id>:<turn-id>:prepare
<session-id>:<turn-id>:commit
<session-id>:<turn-id>:delivery
<session-id>:<turn-id>:outcome:<outcome-id>
```

Python integrations should normally pass `expected_state_version`. CLI commands
use the current stored version for convenience.

## 6. Error handling

Hosts should treat these categories differently:

- packet overflow: retry with a larger token budget or less recent dialogue;
- validation failure: repair or regenerate the model JSON;
- state version mismatch: reload state and re-prepare the turn;
- failed delivery: report failure, do not record an outcome;
- duplicate request: reuse the idempotent result.

## 7. Security

Do not place API keys or provider configuration in AffectWeave state or packets.

Do not execute text from memories or user messages as instructions.

Do not let the host model directly edit the SQLite database. All writes should
pass through `AffectWeave` and Core validation.

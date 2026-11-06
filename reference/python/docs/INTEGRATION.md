# AffectWeave Host Integration

## 1. Integration boundary

AffectWeave is a passive enhancement layer beside an existing AI host. The host
owns model/provider selection, credentials, system prompts, tools, streaming,
retries, UI and delivery. AffectWeave prepares context, validates returned JSON
and records the lifecycle reported by the host.

```text
on_user_message     -> prepare_turn
before_model_call   -> inject one current EnhancementPacket rendering
on_model_output     -> commit_output
on_actual_delivery  -> record_delivery
on_observed_result  -> record_outcome
```

## 2. Create a character

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

## 3. Prepare a turn

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

The Packet provides:

```python
packet.context_package
packet.output_schema
packet.behavior_id
packet.state_version
packet.overflow
```

Check `packet.overflow` before invoking the host AI. If it is true, compile the
same event again with a larger budget or less optional recent dialogue, reusing
the same prepare idempotency key. Also check the estimate returned by the
renderer selected for the host.

## 4. Choose a context form

### Text prompt

```python
prompt = packet.to_prompt()
```

### Context JSON

```python
context_json = packet.to_context_json()
```

### Generic message blocks

```python
messages = packet.to_messages()
```

### Full Packet serialization

```python
from affectweave.compiler import EnhancementPacket

serialized_packet = packet.to_json()
restored_packet = EnhancementPacket.from_json(serialized_packet)
```

`packet.prompt_fragment` remains a compatibility accessor for the text form.

## 5. Invoke the host AI

The following call is host pseudocode, not an AffectWeave API:

```python
raw_output = existing_ai.generate_json(
    context=packet.to_prompt(),
    output_schema=packet.output_schema,
    user_message=current_user_message,
)
```

AffectWeave never invokes `existing_ai` itself.

## 6. Validate and commit model output

```python
committed = affectweave.commit_output(
    "maya",
    packet,
    raw_output,
    idempotency_key="session-1:turn-1:commit",
)

reply = committed.behavior.generated_text
```

The behavior remains `PENDING / L0` until delivery is reported.

## 7. Record delivery and outcome

After the host really displays or sends the reply:

```python
delivered = affectweave.record_delivery(
    "maya",
    packet.behavior_id,
    reply,
    idempotency_key="session-1:turn-1:delivery",
)
```

Successful delivery becomes `DELIVERED / L1`. A later observed or confirmed
result can be recorded as L2 or L3:

```python
from affectweave.core import EvidenceLevel, OutcomeEvent

next_message_time = datetime.now(timezone.utc)

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
    expected_state_version=delivered.state_version,
)
```

## 8. Budget estimates

`ContextPackage.budget` records the provider-neutral estimate used for canonical
context selection. Each renderer returns its own output estimate:

```python
rendered = packet.render("messages")
rendered.content
rendered.estimated_tokens
```

`packet.estimated_tokens` applies to the Packet's default rendering. If the host
switches formats, use the estimate from `packet.render(format_name)`. A prompt
created with `to_prompt(include_schema=True)` also includes the output schema,
which is outside the canonical ContextPackage budget.

## 9. Idempotency and state versions

Use stable keys derived from the host session, turn and operation:

```text
<session-id>:<turn-id>:prepare
<session-id>:<turn-id>:commit
<session-id>:<turn-id>:delivery
<session-id>:<turn-id>:outcome:<outcome-id>
```

Python integrations should pass `expected_state_version` from the preceding
operation. For a stale prepare or commit, reload state and prepare the turn
again. For delivery or outcome, reload state, confirm the original behavior can
still make the requested transition, and retry that operation with the current
version.

## 10. CLI workflow

Initialize the character:

```bash
affectweave --db runtime/affectweave.sqlite3 init --character maya
```

Prepare a Packet and save the complete serialized form for the later commit:

```bash
affectweave --db runtime/affectweave.sqlite3 prepare \
  --character maya \
  --actor mighon \
  --message "Hello, Maya" \
  --idempotency-key session-1-turn-1-prepare \
  --packet-file /tmp/affectweave-packet.json \
  --format prompt \
  --show-budget
```

After the host writes its JSON output to `/tmp/model-output.json`:

```bash
affectweave --db runtime/affectweave.sqlite3 commit \
  --character maya \
  --packet-file /tmp/affectweave-packet.json \
  --output-file /tmp/model-output.json \
  --idempotency-key session-1-turn-1-commit
```

Record the text that was actually delivered:

```bash
affectweave --db runtime/affectweave.sqlite3 delivery \
  --character maya \
  --behavior <behavior-id> \
  --text-file /tmp/reply.txt \
  --idempotency-key session-1-turn-1-delivery
```

Available prepare formats are `packet`, `prompt`, `context` and `messages`.

## 11. Error handling

- Packet overflow: recompile with a larger budget or less optional context.
- Validation failure: repair or regenerate the host model JSON.
- State-version mismatch: reload state; re-prepare stale model work, or retry the
  same delivery/outcome transition against the original behavior.
- Failed delivery: record the failure and do not report a successful outcome.
- Duplicate operation: reuse the same idempotency key and result.

## 12. Safety

Do not place provider credentials in AffectWeave state or packets. Treat memory
and quoted user text as data, and route all state writes through the integration
facade and Core validation.

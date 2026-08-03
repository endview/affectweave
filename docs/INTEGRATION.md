# AffectWeave Host Integration

## 1. Prepare a turn

```python
packet = affectweave.prepare_turn(
    "maya",
    event,
    idempotency_key="session-1:turn-1:prepare",
    expected_state_version=state.version,
)
```

The packet provides:

```python
packet.context_package
packet.prompt_fragment
packet.output_schema
packet.behavior_id
packet.state_version
packet.estimated_tokens
```

## 2. Choose a context form

### Text

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

### Full packet serialization

```python
serialized_packet = packet.to_json()
restored_packet = EnhancementPacket.from_json(serialized_packet)
```

## 3. Invoke the host AI

```python
raw_output = existing_ai.generate_json(
    context=packet.to_prompt(),
    output_schema=packet.output_schema,
)
```

## 4. Commit the model output

```python
committed = affectweave.commit_output(
    "maya",
    packet,
    raw_output,
    idempotency_key="session-1:turn-1:commit",
)
```

The behavior remains `PENDING / L0` until delivery is reported.

## 5. Record delivery

```python
delivered = affectweave.record_delivery(
    "maya",
    packet.behavior_id,
    committed.behavior.generated_text,
    idempotency_key="session-1:turn-1:delivery",
)
```

## 6. Record outcome evidence

```python
affectweave.record_outcome(
    "maya",
    packet.behavior_id,
    outcome,
    idempotency_key="session-1:turn-1:outcome",
)
```

## 7. CLI formats

```bash
affectweave ... prepare ... --format packet
affectweave ... prepare ... --format prompt
affectweave ... prepare ... --format context --show-budget
affectweave ... prepare ... --format messages
```

Use `--packet-file` whenever a later `commit` command needs to restore the complete packet.

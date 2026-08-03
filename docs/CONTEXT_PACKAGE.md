# ContextPackage

`ContextPackage` is the canonical host-neutral view produced for one turn.

## Fields

```text
schema_version
context_version
character_id
turn_id
behavior_id
state_version
immutable_rules
current_event
character
current_state
relationship
active_threads
relevant_memories
recent_dialogue
response_policy
output_contract
budget
trace
```

## Current event

The current event is always retained and is never silently truncated by the context compiler.

## Character projection

The package contains only the character information selected for the current turn. Optional voice notes, contradictions and preferences may be removed when the budget is tight.

## Active threads

ActiveThread base fields remain available during trimming:

```text
id
kind
summary
importance
unresolvedness
```

Optional details such as completion conditions and related entities may be removed when necessary.

## Budget report

```python
package.budget.maximum_tokens
package.budget.estimated_tokens
package.budget.overflow
package.budget.included_sections
package.budget.removed_sections
package.budget.removal_reasons
```

This report describes the provider-neutral estimate used while selecting the
canonical context, including configured renderer overhead. It does not claim an
exact provider token count for every serialized form.

## Trace

`ContextTrace` identifies the compiler version, source event and source state version used to build the package.

## Serialization

```python
payload = package.to_dict()
restored = ContextPackage.from_dict(payload)
```

## Rendering

```python
from affectweave.compiler import (
    JSONRenderer,
    MessageBlockRenderer,
    TextPromptRenderer,
)

text_rendering = TextPromptRenderer().render(package)
json_rendering = JSONRenderer().render(package)
message_rendering = MessageBlockRenderer().render(package)

text = text_rendering.content
json_context = json_rendering.content
messages = message_rendering.content
estimated_tokens = message_rendering.estimated_tokens
```

Every renderer returns a `RenderedContext` with `format`, `content`,
`estimated_tokens` and `renderer_version`.

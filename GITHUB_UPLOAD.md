# Publish AffectWeave to GitHub

This guide publishes AffectWeave as a brand-new public repository.

## 1. Prerequisites

Install Git and GitHub CLI, then authenticate:

```bash
gh auth login
gh auth status
```

## 2. Create the repository

Recommended repository name:

```text
affectweave
```

Using GitHub CLI:

```bash
gh repo create affectweave \
  --public \
  --description "Persistent affect, relationships, and character continuity for AI systems." \
  --source . \
  --remote origin
```

The repository already includes README, MIT License, `.gitignore`, issue templates,
GitHub Actions, contribution guidance, and release notes.

## 3. Push `main`

```bash
git push -u origin main
```

## 4. Push the demo tag

```bash
git push origin v0.1-demo
```

## 5. Create the first release

Attach the wheel from the submission package:

```bash
gh release create v0.1-demo \
  release-assets/affectweave-0.1.0.dev0-py3-none-any.whl \
  --title "AffectWeave v0.1-demo" \
  --notes-file RELEASE_NOTES.md
```

## 6. Configure repository metadata

Suggested GitHub description:

```text
Persistent affect, relationships, and character continuity for AI systems.
```

Suggested topics:

```text
llm
affective-computing
ai-character
emotion
memory
character-continuity
prompt-engineering
state-management
chatbot
roleplay
```

Using GitHub CLI:

```bash
gh repo edit --description "Persistent affect, relationships, and character continuity for AI systems."

gh repo edit --add-topic llm \
  --add-topic affective-computing \
  --add-topic ai-character \
  --add-topic emotion \
  --add-topic memory \
  --add-topic character-continuity \
  --add-topic prompt-engineering \
  --add-topic state-management \
  --add-topic chatbot \
  --add-topic roleplay
```

## 7. Verify the publication

Check:

```bash
gh repo view --web
gh release view v0.1-demo
gh run list --limit 5
```

Confirm that:

- `main` is the default branch;
- the `v0.1-demo` tag is visible;
- the first release contains the wheel;
- GitHub Actions completes the Python test matrix;
- README badges and links render correctly.

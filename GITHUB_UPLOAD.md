# Sync AffectWeave v0.2-demo to GitHub

Target repository: [endview/affectweave](https://github.com/endview/affectweave)

The public repository already contains `v0.1-demo`. Publish `v0.2-demo` as a
normal fast-forward and keep both release tags unchanged.

## 1. Verify the release baseline

```bash
git status --short --branch
git rev-parse v0.2-demo^{commit}
python -m unittest discover -s tests -v
python examples/basic_host.py
python examples/rendering_formats.py
```

Expected release commit:

```text
69ae0c3c7f7718b0b4eaa834696e49294dd49950
```

## 2. Connect the GitHub remote

```bash
gh auth status
```

A repository restored from the migration bundle has a local bundle path as
`origin`. Preserve it as a migration archive and add GitHub separately:

```bash
git remote rename origin migration-bundle
git remote add github https://github.com/endview/affectweave.git
git fetch github refs/heads/main:refs/remotes/github/main
git ls-remote --tags github
```

If the GitHub remote already exists, inspect it instead of adding it again:

```bash
git remote -v
```

## 3. Compare before publishing

```bash
git merge-base --is-ancestor github/main v0.2-demo
git log --oneline --graph --decorate --all -12
git diff --stat github/main..v0.2-demo
```

The ancestor check must succeed. Do not force-push a diverged remote.

## 4. Publish the release commit and tag

Push the exact tagged commit first, then the annotated tag:

```bash
git push github 69ae0c3c7f7718b0b4eaa834696e49294dd49950:refs/heads/main
git push github refs/tags/v0.2-demo:refs/tags/v0.2-demo
```

Documentation-only commits created after the release tag can then advance
`main` without moving `v0.2-demo`:

```bash
git push -u github main
```

## 5. Create the GitHub Release

Use the existing `v0.2-demo` tag; do not ask the Release UI to create a new tag.
The wheel is stored in the migration package outside the Git repository:

```text
release-assets/affectweave-0.2.0.dev0-py3-none-any.whl
```

From the Git repository root, use the current `RELEASE_NOTES.md` and point to the
wheel in the adjacent migration package:

```bash
gh release create v0.2-demo \
  ../AffectWeave_Codex_Migration_v0.2-demo/release-assets/affectweave-0.2.0.dev0-py3-none-any.whl \
  --repo endview/affectweave \
  --title "AffectWeave v0.2-demo" \
  --notes-file RELEASE_NOTES.md
```

## 6. Verify the publication

```bash
gh repo view endview/affectweave --web
gh release view v0.2-demo --repo endview/affectweave
gh run list --repo endview/affectweave --limit 5
```

Confirm that:

- `main` contains the v0.2 Context Pipeline;
- `v0.1-demo` still resolves to `0f49aa3`;
- `v0.2-demo` resolves to `69ae0c3`;
- the release contains `affectweave-0.2.0.dev0-py3-none-any.whl`;
- GitHub Actions passes on Python 3.11, 3.12 and 3.13;
- README, Mermaid diagrams and documentation links render correctly.

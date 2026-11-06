#!/usr/bin/env python3
"""Build one optional Go extension archive; no Python runtime is shipped."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import time
import zipfile


def run(args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', required=True, choices=['windows-amd64', 'linux-amd64'])
    parser.add_argument('--out', required=True)
    parser.add_argument('--go', default='go')
    parser.add_argument('--allow-dirty', action='store_true')
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    out = Path(args.out).resolve()
    commit = run(['git', 'rev-parse', 'HEAD'], cwd=repo)
    dirty = bool(run(['git', 'status', '--porcelain'], cwd=repo))
    if dirty and not args.allow_dirty:
        raise SystemExit('Commit the tested source first, or use --allow-dirty for local validation.')
    manifest = json.loads((repo / 'extension.json').read_text(encoding='utf-8'))
    version = manifest['version']
    name = f'AffectWeave-{version}-{args.target}'
    root = out / name
    root.mkdir(parents=True, exist_ok=False)
    extension = root / manifest['id']
    extension.mkdir()
    binary_name = 'affectweave' + ('.exe' if args.target.startswith('windows') else '')
    manifest['run']['command'] = './' + binary_name
    (extension / 'extension.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    env = dict(os.environ, GOOS=args.target.split('-')[0], GOARCH='amd64', CGO_ENABLED='0')
    subprocess.run([args.go, 'build', '-buildvcs=false', '-trimpath', '-ldflags=-s -w -buildid=', '-o', str(extension / binary_name), './cmd/affectweave'], cwd=repo, env=env, check=True)
    (extension / binary_name).chmod(0o755)
    shutil.copyfile(repo / 'LICENSE', extension / 'LICENSE')
    shutil.copyfile(repo / 'README.md', root / 'README.md')
    modules = run([args.go, 'list', '-deps', '-f', '{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}', './cmd/affectweave'], cwd=repo)
    notices = []
    for line in sorted(set(modules.splitlines())):
        if not line.strip():
            continue
        parts = line.split('\t')
        if len(parts) != 3 or parts[0] == 'github.com/endview/affectweave':
            continue
        module, module_version, directory = parts
        license_path = next((Path(directory) / n for n in ['LICENSE', 'LICENSE.txt', 'LICENSE.md'] if (Path(directory) / n).is_file()), None)
        if not license_path:
            raise RuntimeError('Runtime dependency license missing: ' + module)
        notices.append(f'## {module} {module_version}\n\n{license_path.read_text(encoding="utf-8")}\n')
    goroot = Path(run([args.go, 'env', 'GOROOT'], cwd=repo))
    notices.append('## Go runtime\n\n' + (goroot / 'LICENSE').read_text(encoding='utf-8'))
    (extension / 'THIRD_PARTY_NOTICES.md').write_text('# Runtime licenses\n\n' + '\n'.join(notices), encoding='utf-8')
    files = {str(p.relative_to(root)).replace('\\', '/'): hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file()}
    (root / 'package.json').write_text(json.dumps({'version': version, 'target': args.target, 'commit': commit, 'dirty': dirty, 'files': files}, indent=2) + '\n', encoding='utf-8')
    epoch = int(run(['git', 'show', '-s', '--format=%ct', 'HEAD'], cwd=repo))
    archive = out / (name + ('.zip' if args.target.startswith('windows') else '.tar.gz'))
    if args.target.startswith('windows'):
        stamp = time.gmtime(max(epoch, 315532800))[:6]
        with zipfile.ZipFile(archive, 'x', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as target:
            for p in sorted(root.rglob('*')):
                if not p.is_file():
                    continue
                info = zipfile.ZipInfo(name + '/' + p.relative_to(root).as_posix(), date_time=stamp)
                info.external_attr = (0o755 if p.name == binary_name else 0o644) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                target.writestr(info, p.read_bytes())
    else:
        with archive.open('xb') as raw, gzip.GzipFile(fileobj=raw, filename='', mode='wb', mtime=0) as compressed, tarfile.open(fileobj=compressed, mode='w') as target:
            for p in sorted(root.rglob('*')):
                if not p.is_file():
                    continue
                info = target.gettarinfo(str(p), arcname=name + '/' + p.relative_to(root).as_posix())
                info.mtime, info.uid, info.gid, info.uname, info.gname = epoch, 0, 0, '', ''
                info.mode = 0o755 if p.name == binary_name else 0o644
                with p.open('rb') as f:
                    target.addfile(info, f)
    print(json.dumps({'archive': str(archive), 'extension': str(extension), 'dirty': dirty}))


if __name__ == '__main__':
    main()

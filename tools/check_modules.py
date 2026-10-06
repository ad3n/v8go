#!/usr/bin/env python3
"""Check module distribution through an offline proxy, without local replaces."""
import argparse
import os
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile

MAX_SIZE = 500 * 1024 * 1024
ROOT_MODULE = "github.com/ad3n/v8go"
ROOT = Path(__file__).resolve().parent.parent


def module_files(directory):
    for path in sorted(directory.rglob("*")):
        relative = path.relative_to(directory)
        if any(part in (".git", "vendor", "__pycache__") for part in relative.parts):
            continue
        if path.is_symlink() or not path.is_file():
            continue
        if any((parent / "go.mod").is_file() for parent in path.parents
               if parent != directory and directory in parent.parents):
            continue
        yield relative.as_posix(), path
    if directory != ROOT and not (directory / "LICENSE").exists():
        yield "LICENSE", ROOT / "LICENSE"


def check_size(files):
    total = sum(path.stat().st_size for _, path in files)
    if total > MAX_SIZE:
        raise ValueError(f"module source tree too large: {total} > {MAX_SIZE}")
    for name, path in files:
        limit = 16 * 1024 * 1024 if name in ("go.mod", "LICENSE") else MAX_SIZE
        if path.stat().st_size > limit:
            raise ValueError(f"file too large: {name}")
    return total


def write_module(proxy, module, version, directory):
    files = list(module_files(directory))
    total = check_size(files)
    destination = proxy / module / "@v"
    destination.mkdir(parents=True, exist_ok=True)
    (destination / f"{version}.mod").write_bytes((directory / "go.mod").read_bytes())
    (destination / f"{version}.info").write_text(
        '{"Version":"' + version + '","Time":"2026-10-06T00:00:00Z"}\n')
    (destination / "list").write_text(version + "\n")
    archive = destination / f"{version}.zip"
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as output:
        for name, path in files:
            output.write(path, f"{module}@{version}/{name}")
    if archive.stat().st_size > MAX_SIZE:
        raise ValueError(f"compressed module too large: {module}")
    print(f"{module}@{version}: {total} bytes uncompressed", flush=True)


def create_proxy(proxy, root_version):
    requirements = re.findall(r"(github\.com/ad3n/v8go/deps/\w+)\s+(v\S+)",
                              (ROOT / "go.mod").read_text())
    platforms = sorted(ROOT.glob("deps/*_*/go.mod"))
    required = {module for module, _ in requirements}
    expected = {ROOT_MODULE + "/" + path.parent.relative_to(ROOT).as_posix() for path in platforms}
    if required != expected or len(requirements) != len(platforms) or not requirements:
        raise ValueError("every platform module must be required by the root module")
    for module, version in requirements:
        write_module(proxy, module, version, ROOT / module.removeprefix(ROOT_MODULE + "/"))
    root_files = list(module_files(ROOT))
    if any(re.match(r"deps/\w+_(amd64|arm64)/", name) for name, _ in root_files):
        raise ValueError("platform archives leaked into the root module")
    write_module(proxy, ROOT_MODULE, root_version, ROOT)


def verify(proxy, root_version, work):
    env = dict(os.environ, GOWORK="off", GOFLAGS="", GOSUMDB="off",
               GOPROXY=proxy.as_uri(), GOMODCACHE=str(work / "modcache"))
    consumer = work / "consumer"
    consumer.mkdir()
    (consumer / "go.mod").write_text(
        f"module example.com/modulecheck\n\ngo 1.26\n\nrequire {ROOT_MODULE} {root_version}\n")
    (consumer / "main.go").write_text('''package main

import (
    "fmt"
    v8 "github.com/ad3n/v8go"
)

func main() {
    ctx := v8.NewContext()
    defer ctx.Isolate().Dispose()
    defer ctx.Close()
    value, err := ctx.RunScript("3 + 4", "modulecheck.js")
    if err != nil {
        panic(err)
    }
    fmt.Println(value)
    value.Release()
}
''')
    def run(*command):
        return subprocess.run(command, cwd=consumer, env=env, check=True,
                              text=True, stdout=subprocess.PIPE).stdout
    run("go", "mod", "tidy")
    run("go", "mod", "download", "all")
    for mode in ("mod", "vendor"):
        if mode == "vendor":
            run("go", "mod", "vendor")
        if run("go", "run", f"-mod={mode}", ".").strip() != "7":
            raise ValueError(f"unexpected result in {mode} mode")
        print(f"downloaded {mode} build OK", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--proxy-dir", type=Path)
    parser.add_argument("--root-version", default="v1.1.5")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="v8go-modulecheck-") as temporary:
        work = Path(temporary)
        proxy = args.proxy_dir.resolve() if args.proxy_dir else work / "proxy"
        create_proxy(proxy, args.root_version)
        verify(proxy, args.root_version, work)


if __name__ == "__main__":
    main()

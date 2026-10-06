import importlib.util
from pathlib import Path
import tempfile
import unittest
import subprocess
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location("check_modules", Path(__file__).with_name("check_modules.py"))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class ModuleTests(unittest.TestCase):
    def test_module_keeps_headers_and_excludes_local_native_builds(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "source"
            root.mkdir()
            (root / "go.mod").write_text("module github.com/ad3n/v8go\n\ngo 1.26\n")
            header = root / "deps/include/v8.h"
            header.parent.mkdir(parents=True)
            header.write_text("header")
            native = root / ".build/native/linux_amd64/libv8-0.a"
            native.parent.mkdir(parents=True)
            native.write_bytes(b"native")
            proxy = Path(temporary) / "proxy"
            with patch.object(checker, "ROOT", root):
                checker.create_proxy(proxy, "v0.0.0")
            with zipfile.ZipFile(proxy / "github.com/ad3n/v8go/@v/v0.0.0.zip") as archive:
                self.assertTrue(any(name.endswith("/deps/include/v8.h") for name in archive.namelist()))
                self.assertFalse(any(name.endswith(".a") for name in archive.namelist()))
            (root / "native.a").write_bytes(b"native")
            with patch.object(checker, "ROOT", root):
                with self.assertRaisesRegex(ValueError, "native binaries"):
                    checker.create_proxy(proxy, "v0.0.0")

    def test_generator_links_external_libraries_and_excludes_android_from_linux(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for platform in ("linux_amd64", "android_amd64"):
                target = root / ".build" / "native" / platform
                target.mkdir(parents=True)
                (target / "libmanifest").write_text("libv8-0.a\n")
            subprocess.run(["python3", str(checker.ROOT / "deps/update_cgo.py")], cwd=root, check=True)
            linux = (root / "cgo_linux_amd64.go").read_text()
            self.assertIn("cgo && linux && !android && amd64", linux)
            self.assertNotIn("github.com/ad3n/v8go/deps/", linux)
            self.assertNotIn("-L", linux)
            self.assertIn("-Wl,--start-group -lv8-0 -Wl,--end-group", linux)
            for platform in ("linux_amd64", "android_amd64"):
                target = root / ".build" / "native" / platform
                self.assertFalse((target / "go.mod").exists())
                self.assertFalse((target / "cgo.go").exists())

    def test_nested_modules_and_vendor_are_excluded(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "go.mod").write_text("module example.com/root\n")
            for directory in ("deps/linux_amd64", "vendor/example.com/lib", ".git"):
                target = root / directory
                target.mkdir(parents=True)
                (target / "large.a").write_bytes(b"native")
            (root / "deps/linux_amd64/go.mod").write_text("module example.com/native\n")
            names = {name for name, _ in checker.module_files(root)}
            self.assertEqual(names, {"go.mod", "LICENSE"})

    def test_size_boundaries(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "archive.a"
            self.assertEqual(checker.check_size([]), 0)
            for size in (0, checker.MAX_SIZE, checker.MAX_SIZE + 1):
                with path.open("wb") as output:
                    output.truncate(size)
                if size <= checker.MAX_SIZE:
                    self.assertEqual(checker.check_size([("archive.a", path)]), size)
                else:
                    with self.assertRaises(ValueError):
                        checker.check_size([("archive.a", path)])


if __name__ == "__main__":
    unittest.main()

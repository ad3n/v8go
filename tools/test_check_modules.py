import importlib.util
from pathlib import Path
import tempfile
import unittest
import subprocess

spec = importlib.util.spec_from_file_location("check_modules", Path(__file__).with_name("check_modules.py"))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class ModuleTests(unittest.TestCase):
    def test_generator_uses_platform_modules_and_excludes_android_from_linux(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for platform in ("linux_amd64", "android_amd64"):
                target = root / "deps" / platform
                target.mkdir(parents=True)
                (target / "libmanifest").write_text("libv8-0.a\n")
            subprocess.run(["python3", str(checker.ROOT / "deps/update_cgo.py")], cwd=root, check=True)
            linux = (root / "cgo_linux_amd64.go").read_text()
            self.assertIn("cgo && linux && !android && amd64", linux)
            self.assertIn('import _ "github.com/ad3n/v8go/deps/linux_amd64"', linux)
            self.assertNotIn("LDFLAGS", linux)
            for platform in ("linux_amd64", "android_amd64"):
                target = root / "deps" / platform
                self.assertIn("module github.com/ad3n/v8go/deps/" + platform,
                              (target / "go.mod").read_text())
                self.assertIn("-L${SRCDIR}", (target / "cgo.go").read_text())
                self.assertIn("-Wl,--start-group -lv8-0 -Wl,--end-group",
                              (target / "cgo.go").read_text())

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

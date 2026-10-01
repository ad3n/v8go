import pathlib
import subprocess
import sys
import tempfile
import unittest


class LocalLinkerTest(unittest.TestCase):
    def test_local_linker_generation(self):
        generator = pathlib.Path(__file__).with_name("update_cgo.py")
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            for target in ("darwin_arm64", "linux_amd64", "android_arm64"):
                architecture = root / "deps" / target
                architecture.mkdir(parents=True)
                (architecture / "libmanifest").write_text("libv8-0.a\nlibv8-1.a\n")

            command = [sys.executable, str(generator)]
            subprocess.run(command, cwd=root, check=True)
            generated = {}
            for target in ("darwin_arm64", "linux_amd64", "android_arm64"):
                source = root / ("cgo_" + target + ".go")
                text = source.read_text()
                generated[target] = text
                self.assertIn("-L${SRCDIR}/deps/" + target, text)
                self.assertIn('import "C"', text)
                self.assertNotIn("tommie", text)
                self.assertNotIn('import _', text)
                self.assertIn("-framework CoreFoundation", text)
                if target.startswith("darwin"):
                    self.assertNotIn("--start-group", text)
                else:
                    self.assertIn("-Wl,--start-group -lv8-0 -lv8-1 -Wl,--end-group", text)
                self.assertIn("github.com/ad3n/v8go/deps/" + target,
                              (root / "deps" / target / "go.mod").read_text())

            subprocess.run(command, cwd=root, check=True)
            for target, text in generated.items():
                self.assertEqual(text, (root / ("cgo_" + target + ".go")).read_text())


if __name__ == "__main__":
    unittest.main()

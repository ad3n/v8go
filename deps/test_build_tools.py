import pathlib
import runpy
import sys
import unittest
from unittest import mock


class BuildToolTest(unittest.TestCase):
    def test_native_tools_do_not_require_depot_python_bootstrap(self):
        script = pathlib.Path(__file__).with_name("build.py")
        with mock.patch.object(sys, "argv", [str(script)]):
            namespace = runpy.run_path(str(script))

        select = namespace["native_build_tools"]
        source = pathlib.Path(namespace["v8_path"])
        for host, directory, suffix in (("Linux", "linux64", ""),
                                        ("Darwin", "mac", ""),
                                        ("Windows", "win", ".exe")):
            with self.subTest(host=host), mock.patch("platform.system", return_value=host):
                gn, ninja = select()
                self.assertEqual(pathlib.Path(gn), source / "buildtools" / directory / ("gn" + suffix))
                self.assertEqual(pathlib.Path(ninja), source / "third_party" / "ninja" / ("ninja" + suffix))
                self.assertNotIn("depot_tools", gn)
                self.assertNotIn("depot_tools", ninja)


if __name__ == "__main__":
    unittest.main()

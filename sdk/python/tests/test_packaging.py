"""The two versions that must never disagree.

A wheel whose metadata says 0.1.0 while the module reports something else produces manifests that
name the wrong SDK version, and the manifest is the artefact people quote.
"""

import os
import re
import unittest

from context_ci import __version__

try:
    import tomllib
except ModuleNotFoundError:  # Python 3.10 and older
    tomllib = None

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


@unittest.skipIf(tomllib is None, "tomllib needs Python 3.11 or newer")
class PackagingTest(unittest.TestCase):
    def test_version_matches_pyproject(self):
        with open(os.path.join(ROOT, "pyproject.toml"), "rb") as handle:
            declared = tomllib.load(handle)["project"]["version"]
        self.assertEqual(declared, __version__)

    def test_the_package_has_no_runtime_dependencies(self):
        with open(os.path.join(ROOT, "pyproject.toml"), "rb") as handle:
            project = tomllib.load(handle)["project"]
        # No dependencies key at all is the point: the capture file is the whole interface, and an
        # SDK that drags a tree of packages into a user's build is not one they will accept.
        self.assertNotIn("dependencies", project)

    def test_the_package_name_is_the_one_the_manifest_reports(self):
        with open(os.path.join(ROOT, "pyproject.toml"), "rb") as handle:
            name = tomllib.load(handle)["project"]["name"]
        self.assertEqual(name, "context-ci-capture")

    def test_the_readme_documents_the_install(self):
        with open(os.path.join(ROOT, "README.md"), encoding="utf-8") as handle:
            readme = handle.read()
        self.assertIsNotNone(re.search(r"pip install", readme), "README does not say how to install")


if __name__ == "__main__":
    unittest.main()

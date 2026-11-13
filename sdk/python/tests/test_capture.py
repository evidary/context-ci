"""Tests for the capture SDK.

Run with:  python -m unittest discover -s sdk/python/tests
"""

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from context_ci import CAPTURE_VERSION, CaptureError, capture, capture_messages, __version__


class CaptureTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.mkdtemp()
        self.path = os.path.join(self.directory, "context-capture.json")

    def read(self):
        with open(self.path, encoding="utf-8") as handle:
            return json.load(handle)

    def test_writes_the_documented_document(self):
        capture("You are a support agent.", path=self.path, template="You are a {{role}}.")
        document = self.read()
        self.assertEqual(document["capture_version"], CAPTURE_VERSION)
        self.assertEqual(document["sdk"], "context-ci-python")
        self.assertEqual(document["sdk_version"], __version__)
        self.assertEqual(document["rendered"], "You are a support agent.")
        self.assertEqual(document["template"], "You are a {{role}}.")

    def test_returns_the_path_it_wrote(self):
        self.assertEqual(capture("text", path=self.path), self.path)

    def test_template_is_optional(self):
        capture("text", path=self.path)
        self.assertNotIn("template", self.read())

    def test_refuses_an_empty_rendered_prompt(self):
        for bad in ("", None, 42):
            with self.assertRaises(CaptureError):
                capture(bad, path=self.path)
        self.assertFalse(os.path.exists(self.path))

    def test_refuses_a_non_string_template(self):
        with self.assertRaises(CaptureError):
            capture("text", path=self.path, template={"not": "a string"})

    def test_refuses_a_prompt_over_the_limit(self):
        with self.assertRaises(CaptureError) as caught:
            capture("x" * ((4 << 20) + 1), path=self.path)
        self.assertIn("over the", str(caught.exception))

    def test_creates_the_directory(self):
        nested = os.path.join(self.directory, "a", "b", "capture.json")
        capture("text", path=nested)
        self.assertTrue(os.path.exists(nested))

    def test_leaves_no_temporary_file_behind(self):
        capture("text", path=self.path)
        self.assertEqual(sorted(os.listdir(self.directory)), ["context-capture.json"])

    def test_writes_lf_endings(self):
        capture("text", path=self.path)
        with open(self.path, "rb") as handle:
            self.assertNotIn(b"\r\n", handle.read())

    def test_messages_are_joined_the_same_way_every_time(self):
        capture_messages(
            [{"role": "system", "content": "be brief"}, {"role": "user", "content": "hi"}],
            path=self.path,
        )
        self.assertEqual(self.read()["rendered"], "system: be brief\nuser: hi")

    def test_messages_refuse_an_empty_list(self):
        with self.assertRaises(CaptureError):
            capture_messages([], path=self.path)


if __name__ == "__main__":
    unittest.main()
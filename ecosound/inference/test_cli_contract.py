"""CLI output isolation tests; model execution is mocked only in this test file."""

import contextlib
import importlib.util
import io
import json
import subprocess
import sys
import unittest
from pathlib import Path
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("identify.py")
SPEC = importlib.util.spec_from_file_location("ecsound_cli", SCRIPT)
identify = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(identify)


class CLIContractTests(unittest.TestCase):
    def test_success_has_exit_zero_and_exactly_one_json_object(self):
        fixture = {"model": "birdnet", "model_version": "3.0", "predictions": []}
        output = io.StringIO()
        with patch.object(identify, "identify", return_value=fixture) as infer:
            with contextlib.redirect_stdout(output):
                self.assertEqual(identify.main(["test-decoded.wav"]), 0)
        infer.assert_called_once()
        self.assertEqual(json.loads(output.getvalue()), fixture)
        self.assertEqual(output.getvalue().count("\n"), 1)

    def test_inference_exception_has_exit_one_and_no_success_json(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        with patch.object(
            identify, "identify", side_effect=RuntimeError("model unavailable")
        ):
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                self.assertEqual(identify.main(["test-decoded.wav"]), 1)
        self.assertEqual(stdout.getvalue(), "")
        self.assertIn("model unavailable", stderr.getvalue())

    def test_python_and_native_diagnostics_never_pollute_stdout(self):
        code = (
            "import os,sys; sys.path.insert(0, "
            + repr(str(SCRIPT.parent.resolve()))
            + "); "
            "import identify\n"
            "with identify.diagnostics_to_stderr():\n"
            "    print('Python model diagnostic')\n"
            "    os.write(1, b'Native model diagnostic\\n')\n"
            "print('{\"predictions\": []}')\n"
        )
        run = subprocess.run(
            [sys.executable, "-c", code], capture_output=True, text=True, check=False
        )
        self.assertEqual(run.returncode, 0)
        self.assertEqual(json.loads(run.stdout), {"predictions": []})
        self.assertIn("Python model diagnostic", run.stderr)
        self.assertIn("Native model diagnostic", run.stderr)


if __name__ == "__main__":
    unittest.main()

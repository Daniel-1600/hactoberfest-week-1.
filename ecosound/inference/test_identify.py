"""Contract tests for the BirdNET adapter; no model download is required."""

import importlib.util
import subprocess
import sys
import tempfile
import unittest
import wave
from pathlib import Path

SCRIPT = Path(__file__).with_name("identify.py")
SPEC = importlib.util.spec_from_file_location("identify", SCRIPT)
identify = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(identify)


class Rows(list):
    def __init__(self, names, values):
        super().__init__(values)
        self.dtype = type("DType", (), {"names": names})()


class Result:
    def __init__(self, names, values, unprocessable=()):
        self.rows = Rows(names, values)
        self.unprocessable_inputs = unprocessable

    def to_structured_array(self):
        return self.rows


FIELDS = ("species_name", "confidence", "start_time", "end_time")


class NormalizeTests(unittest.TestCase):
    def test_preserves_each_temporal_row_and_original_label(self):
        result = Result(
            FIELDS,
            [
                {
                    "species_name": "Turdus merula_Common_Blackbird",
                    "confidence": 0.8123456789,
                    "start_time": 0,
                    "end_time": 3,
                },
                {
                    "species_name": "Turdus merula_Common_Blackbird",
                    "confidence": 0.91,
                    "start_time": 3,
                    "end_time": 6,
                },
            ],
        )
        data = identify.normalize_predictions(result)
        self.assertEqual((data["model"], data["model_version"]), ("birdnet", "3.0"))
        self.assertEqual(len(data["predictions"]), 2)
        first = data["predictions"][0]
        self.assertEqual(first["raw_species_name"], "Turdus merula_Common_Blackbird")
        self.assertEqual(first["scientific_name"], "Turdus merula")
        self.assertEqual(first["common_name"], "Common_Blackbird")
        self.assertEqual(first["confidence"], 0.8123456789)
        self.assertEqual(data["predictions"][1]["start_time"], 3.0)

    def test_empty_detection_is_a_valid_result(self):
        data = identify.normalize_predictions(Result(FIELDS, []))
        self.assertEqual(data["predictions"], [])

    def test_unprocessable_input_is_an_error_even_with_no_rows(self):
        with self.assertRaisesRegex(ValueError, "could not process"):
            identify.normalize_predictions(Result(FIELDS, [], ["broken.wav"]))

    def test_missing_schema_is_an_error(self):
        with self.assertRaisesRegex(ValueError, "schema"):
            identify.normalize_predictions(Result(("species_name",), []))

    def test_nonfinite_and_invalid_scores_or_times_are_errors(self):
        invalid = [(float("nan"), 0, 1), (1.1, 0, 1), (0.5, 2, 1), (0.5, -1, 1)]
        for score, start, end in invalid:
            with self.subTest(score=score, start=start, end=end):
                with self.assertRaises(ValueError):
                    identify.normalize_predictions(
                        Result(
                            FIELDS,
                            [
                                {
                                    "species_name": "Bird_Name",
                                    "confidence": score,
                                    "start_time": start,
                                    "end_time": end,
                                }
                            ],
                        )
                    )


class AudioAndCLITests(unittest.TestCase):
    def test_valid_mono_pcm_passes_boundary_validation(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "valid.wav"
            with wave.open(str(path), "wb") as audio:
                audio.setnchannels(1)
                audio.setsampwidth(2)
                audio.setframerate(8000)
                audio.writeframes(b"\x00\x00" * 8000)
            identify.validate_audio(path)

    def test_invalid_audio_rejected_before_model_import(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "invalid.wav"
            path.write_bytes(b"not audio")
            with self.assertRaises((ValueError, wave.Error)):
                identify.validate_audio(path)
            run = subprocess.run(
                [sys.executable, str(SCRIPT), str(path)],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(run.returncode, 1)
            self.assertEqual(run.stdout, "")
            self.assertIn("BirdNET inference failed", run.stderr)

    def test_argument_errors_exit_two_and_emit_no_json(self):
        run = subprocess.run(
            [sys.executable, str(SCRIPT)], capture_output=True, text=True, check=False
        )
        self.assertEqual(run.returncode, 2)
        self.assertEqual(run.stdout, "")

    def test_invalid_threshold_exits_one_without_model_load(self):
        run = subprocess.run(
            [sys.executable, str(SCRIPT), "missing.wav", "--threshold", "1.1"],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(run.returncode, 1)
        self.assertEqual(run.stdout, "")
        self.assertIn("Threshold", run.stderr)


if __name__ == "__main__":
    unittest.main()

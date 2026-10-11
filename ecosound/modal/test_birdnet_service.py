"""Request boundary tests run without Modal or BirdNET model downloads."""

import io
import unittest
import wave

from birdnet_service import MAX_AUDIO_BYTES, infer_wav


def make_wav(seconds=1, channels=1):
    output = io.BytesIO()
    with wave.open(output, "wb") as audio:
        audio.setnchannels(channels)
        audio.setsampwidth(2)
        audio.setframerate(32000)
        audio.writeframes(b"\x00\x00" * 32000 * seconds * channels)
    return output.getvalue()


class BirdnetServiceTests(unittest.TestCase):
    def test_valid_wav_passes_through_existing_contract(self):
        expected = {
            "model": "birdnet",
            "model_version": "3.0",
            "predictions": [
                {
                    "raw_species_name": "Turdus merula_Blackbird",
                    "scientific_name": "Turdus merula",
                    "common_name": "Blackbird",
                    "confidence": 0.719,
                    "start_time": 0.0,
                    "end_time": 3.0,
                }
            ],
        }
        observed = []

        def fake_identify(path):
            observed.append(path.read_bytes())
            return expected

        self.assertEqual(infer_wav(make_wav(), fake_identify), expected)
        self.assertEqual(observed, [make_wav()])

    def test_valid_empty_result_is_preserved(self):
        expected = {"model": "birdnet", "model_version": "3.0", "predictions": []}
        self.assertEqual(infer_wav(make_wav(), lambda _: expected), expected)

    def test_invalid_audio_is_rejected_before_inference(self):
        for payload in (b"", b"not wav", make_wav(channels=2), make_wav(seconds=61)):
            with self.subTest(length=len(payload)):
                with self.assertRaises(ValueError):
                    infer_wav(payload, lambda _: self.fail("inference must not run"))

    def test_oversized_audio_is_rejected_before_inference(self):
        with self.assertRaisesRegex(ValueError, "8 MiB"):
            infer_wav(b"x" * (MAX_AUDIO_BYTES + 1), lambda _: self.fail("must reject"))


if __name__ == "__main__":
    unittest.main()

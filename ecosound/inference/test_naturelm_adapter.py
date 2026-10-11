import unittest

from naturelm_adapter import normalize_outputs


class NatureLMAdapterTests(unittest.TestCase):
    def test_descriptions_remain_unverified_text(self):
        result = normalize_outputs(["#0.00s - 10.00s#: A repeated rising call."], "audio_description")
        self.assertEqual(result["provider"], "naturelm-audio")
        self.assertFalse(result["species_identification"])
        self.assertIn("rising call", result["outputs"][0])

    def test_rejects_unsupported_shape_and_tasks(self):
        for outputs, task in [("species", "audio_description"), (["Bird"], "bird_species_classification"), ([""], "audio_description"), (["x" * 2001], "audio_description")]:
            with self.subTest(outputs=outputs, task=task), self.assertRaises(ValueError):
                normalize_outputs(outputs, task)


if __name__ == "__main__":
    unittest.main()

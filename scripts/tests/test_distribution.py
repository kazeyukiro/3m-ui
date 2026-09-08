"""Release channel and pinned download failure regression tests (no network)."""
import importlib.util
from pathlib import Path
import os
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("release_metadata", ROOT / "scripts/release-metadata.py")
metadata = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(metadata)


class DistributionTests(unittest.TestCase):
    def test_stable_and_candidate_channels(self):
        stable = metadata.metadata("v1.2.3")
        self.assertEqual((stable["latest"], stable["prerelease"]), ("true", "false"))
        candidate = metadata.metadata("v1.3.0-rc.1")
        self.assertEqual((candidate["latest"], candidate["prerelease"]), ("false", "true"))

    def test_build_metadata_does_not_collide_with_prerelease_image(self):
        build = metadata.metadata("v1.2.3+rc.1")
        candidate = metadata.metadata("v1.2.3-rc.1")
        self.assertEqual(build["latest"], "true")
        self.assertNotEqual(build["image_tag"], candidate["image_tag"])
        self.assertNotIn("+", build["image_tag"])

    def test_invalid_and_rolling_tags_cannot_be_formal_releases(self):
        for tag in ("pre", "v1.2", "v01.2.3", "v1.2.3-01", "v1.2.3-", "v1.2.3\nlatest=true"):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                metadata.metadata(tag)

    def test_changed_upstream_asset_is_rejected_before_installing_core(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            fake_curl = directory / "curl"
            fake_curl.write_text(
                '#!/bin/sh\nwhile [ "$#" -gt 0 ]; do\n'
                '  if [ "$1" = --output ]; then shift; printf tampered > "$1"; exit; fi\n'
                '  shift\ndone\nexit 1\n', encoding="utf-8"
            )
            fake_curl.chmod(0o755)
            dest = directory / "output"
            env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}")
            result = subprocess.run(
                ["sh", str(ROOT / "scripts/download-mihomo.sh"), "amd64", str(dest)],
                env=env, capture_output=True, text=True, check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("checksum mismatch", result.stderr)
            self.assertFalse((dest / "mihomo").exists())


if __name__ == "__main__":
    unittest.main()

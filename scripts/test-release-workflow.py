#!/usr/bin/env python3
"""Exercise release decisions and the actual workflow scripts without publishing."""

import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("release_plan", ROOT / "scripts/release-plan.py")
RELEASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RELEASE)
WORKFLOW = (ROOT / ".github/workflows/publish.yml").read_text()


def plan(**overrides):
    options = dict(event="workflow_dispatch", ref="refs/heads/main", current_app="0.21.1.3",
                   current_bridge="0.21.1.3", current_beszel="0.21.1", latest_beszel="0.21.2")
    return RELEASE.plan_release(**(options | overrides))


def run_script(name):
    step = WORKFLOW.split("      - name: " + name + "\n", 1)[1]
    body = step.split("        run: |\n", 1)[1]
    lines = []
    for line in body.splitlines():
        if line.strip() and not line.startswith("          "):
            break
        lines.append(line[10:] if line else "")
    return "\n".join(lines) + "\n"


class ReleasePlanTests(unittest.TestCase):
    def test_manual_skip_bridge_updates_other_apps_and_creates_a_release(self):
        result = plan(image_type="all-except-bridge")
        self.assertEqual(result["image_type"], "all-except-bridge")
        self.assertEqual(result["APP_VERSION"], "0.21.2")
        self.assertEqual(result["BESZEL_VERSION"], "0.21.2")
        self.assertEqual(result["BRIDGE_APP_VERSION"], "0.21.1.3")
        self.assertEqual(result["verify_bridge"], "false")
        self.assertEqual(result["RELEASE_TAG"], "v0.21.2")

    def test_bridge_only_new_version_keeps_the_upstream_pin(self):
        result = plan(image_type="bridge", bridge_version="0.21.1.4")
        self.assertEqual(result["APP_VERSION"], "0.21.1.4")
        self.assertEqual(result["BRIDGE_APP_VERSION"], "0.21.1.4")
        self.assertEqual(result["BESZEL_VERSION"], "0.21.1")
        self.assertEqual(result["verify_bridge"], "true")
        self.assertEqual(result["RELEASE_TAG"], "bridge-v0.21.1.4")

    def test_bridge_rebuild_uses_its_own_version(self):
        result = plan(image_type="bridge", current_bridge="0.21.1.4")
        self.assertEqual(result["APP_VERSION"], "0.21.1.4")
        self.assertEqual(result["run_steps"], "true")
        self.assertEqual(result["RELEASE_TAG"], "")

    def test_forcing_a_new_bridge_version_still_creates_its_release(self):
        self.assertEqual(plan(image_type="bridge", bridge_version="0.21.1.4", force=True)["RELEASE_TAG"],
                         "bridge-v0.21.1.4")

    def test_bridge_can_catch_up_after_being_skipped(self):
        result = plan(image_type="bridge", current_app="0.21.2", current_beszel="0.21.2",
                      bridge_version="0.21.2")
        self.assertEqual(result["RELEASE_TAG"], "bridge-v0.21.2")
        self.assertEqual(result["BESZEL_VERSION"], "0.21.2")

    def test_scheduled_runs_ignore_manual_controls(self):
        result = plan(event="schedule", image_type="all-except-bridge", bridge_version="invalid", force=True)
        self.assertEqual(result["image_type"], "all")
        self.assertEqual(result["verify_bridge"], "true")
        self.assertEqual(result["BRIDGE_APP_VERSION"], "0.21.2")
        self.assertEqual(result["RELEASE_TAG"], "v0.21.2")

    def test_tag_runs_ignore_manual_controls_and_keep_the_upstream_pin(self):
        result = plan(event="push", ref="refs/tags/v0.21.1.5", image_type="bridge",
                      bridge_version="invalid", force=True)
        self.assertEqual(result["image_type"], "all")
        self.assertEqual(result["APP_VERSION"], "0.21.1.5")
        self.assertEqual(result["BESZEL_VERSION"], "0.21.1")
        self.assertEqual(result["verify_bridge"], "true")
        self.assertEqual(result["RELEASE_TAG"], "")

    def test_up_to_date_full_runs_skip_unless_forced(self):
        for scope in RELEASE.FULL_SCOPES:
            with self.subTest(scope=scope):
                self.assertEqual(plan(image_type=scope, latest_beszel="0.21.1")["run_steps"], "false")
                forced = plan(image_type=scope, latest_beszel="0.21.1", force=True)
                self.assertEqual(forced["run_steps"], "true")
                self.assertEqual(forced["RELEASE_TAG"], "")

    def test_full_rebuild_preserves_a_newer_bridge_version(self):
        result = plan(current_bridge="0.21.2.1", latest_beszel="0.21.1", force=True)
        self.assertEqual(result["APP_VERSION"], "0.21.1.3")
        self.assertEqual(result["BRIDGE_APP_VERSION"], "0.21.2.1")

    def test_next_upstream_release_advances_bridge(self):
        self.assertEqual(plan(current_bridge="0.21.1.4")["BRIDGE_APP_VERSION"], "0.21.2")

    def test_other_single_app_rebuilds_keep_existing_versions(self):
        for scope in sorted(RELEASE.IMAGE_TYPES - RELEASE.FULL_SCOPES - {"bridge"}):
            with self.subTest(scope=scope):
                result = plan(image_type=scope)
                self.assertEqual(result["APP_VERSION"], "0.21.1.3")
                self.assertEqual(result["BESZEL_VERSION"], "0.21.1")
                self.assertEqual(result["verify_bridge"], "false")
                self.assertEqual(result["RELEASE_TAG"], "")

    def test_invalid_manual_options_are_rejected(self):
        cases = [dict(image_type="unknown"), dict(bridge_version="0.21.1.4"),
                 dict(image_type="bridge", bridge_version="0.20.0"),
                 dict(image_type="bridge", bridge_version="$(touch unwanted)"),
                 dict(image_type="bridge", bridge_version="0.21.2\nRELEASE_TAG=bad"),
                 dict(image_type="bridge", current_bridge="0.21.1", bridge_version="0.21.1.0"),
                 dict(ref="refs/tags/v0.21.2")]
        for options in cases:
            with self.subTest(options=options), self.assertRaises(ValueError):
                plan(**options)

    def test_command_line_writes_workflow_outputs(self):
        with tempfile.TemporaryDirectory(prefix="beszel-release-plan-") as directory:
            output = Path(directory) / "outputs"
            subprocess.run([sys.executable, "-B", str(ROOT / "scripts/release-plan.py"),
                            "--event", "workflow_dispatch", "--ref", "refs/heads/main",
                            "--image-type", "bridge", "--output", str(output)],
                           check=True, capture_output=True, text=True)
            result = dict(line.split("=", 1) for line in output.read_text().splitlines())
            self.assertEqual(result["APP_VERSION"], RELEASE.config_version("beszel_bridge"))
            self.assertEqual(result["BESZEL_VERSION"], (ROOT / "beszel_agent/beszel_version").read_text().strip())
            self.assertEqual(result["verify_bridge"], "true")
            self.assertEqual(result["RELEASE_TAG"], "")


class ReleaseWorkflowTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="beszel-release-workflow-")
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.calls = self.directory / "calls.jsonl"
        self.env = dict(os.environ, CALLS_FILE=str(self.calls), TMPDIR=str(self.directory),
                        GITHUB_REPOSITORY="example/beszel", PATH=str(self.directory) + os.pathsep + os.environ["PATH"])
        for name in ("apps.tsv", "select-apps.sh"):
            body = re.search(r"cat > /tmp/" + re.escape(name) + r" <<'EOF'\n(.*?)\n          EOF", WORKFLOW, re.S)[1]
            target = self.directory / name
            target.write_text(textwrap.dedent(body) + "\n")
            target.chmod(0o755)
        self.apps = {line.split("\t")[0]: line.split("\t")[1]
                     for line in (self.directory / "apps.tsv").read_text().splitlines()}
        for app in self.apps.values():
            directory = self.directory / app
            directory.mkdir()
            (directory / "config.yaml").write_text('version: "0.21.1.3"\n')
            (directory / "CHANGELOG.md").write_text("original notes\n")
            if app != "beszel_bridge":
                (directory / "beszel_version").write_text("0.21.1\n")
        recorder = '''#!/usr/bin/env python3
import json,os,pathlib,sys
call={'command':pathlib.Path(sys.argv[0]).name,'args':sys.argv[1:]}
if '--notes-file' in sys.argv:
    call['notes']=pathlib.Path(sys.argv[sys.argv.index('--notes-file')+1]).read_text()
with open(os.environ['CALLS_FILE'],'a') as output: output.write(json.dumps(call)+'\\n')
'''
        for command in ("build-app-image.sh", "git", "gh"):
            target = self.directory / command
            target.write_text(recorder)
            target.chmod(0o755)

    def execute(self, step, result):
        script = run_script(step)
        for command in ("select-apps.sh", "build-app-image.sh"):
            script = script.replace("/tmp/" + command, str(self.directory / command))
        script = re.sub(r"\$\{\{ steps\.plan\.outputs\.([A-Za-z_]+) \}\}", lambda match: result[match[1]], script)
        script = script.replace("${{ github.ref_name }}", "main")
        completed = subprocess.run(["bash", "-eu", "-o", "pipefail", "-c", script], cwd=self.directory,
                                   env=self.env | result | {"IMAGE_TYPE": result["image_type"]},
                                   capture_output=True, text=True)
        self.assertEqual(completed.returncode, 0, completed.stderr)

    def recorded(self, command):
        return [call for call in map(json.loads, self.calls.read_text().splitlines()) if call["command"] == command]

    def test_skip_bridge_preserves_its_files_and_publishes_only_six_apps(self):
        result = plan(image_type="all-except-bridge")
        self.execute("Update selected app versions", result)
        self.execute("Publish images", result)
        self.execute("Commit app version update", result)
        for scope, app in self.apps.items():
            directory = self.directory / app
            version = "0.21.1.3" if scope == "bridge" else "0.21.2"
            self.assertIn('version: "' + version + '"', (directory / "config.yaml").read_text())
        self.assertEqual((self.directory / "beszel_bridge/CHANGELOG.md").read_text(), "original notes\n")
        builds = self.recorded("build-app-image.sh")
        self.assertEqual(len(builds), 6)
        self.assertNotIn("beszel_bridge", {call["args"][1] for call in builds})
        staged = [call["args"] for call in self.recorded("git") if call["args"][0] == "add"]
        self.assertFalse(any("beszel_bridge/" in arg for args in staged for arg in args))

    def test_bridge_release_changes_and_publishes_only_bridge(self):
        result = plan(image_type="bridge", bridge_version="0.21.1.4")
        self.execute("Update selected app versions", result)
        self.execute("Publish images", result)
        self.execute("Commit app version update", result)
        self.execute("Create GitHub Release", result)
        for scope, app in self.apps.items():
            directory = self.directory / app
            version = "0.21.1.4" if scope == "bridge" else "0.21.1.3"
            self.assertIn('version: "' + version + '"', (directory / "config.yaml").read_text())
            if scope != "bridge":
                self.assertEqual((directory / "beszel_version").read_text(), "0.21.1\n")
                self.assertEqual((directory / "CHANGELOG.md").read_text(), "original notes\n")
        builds = self.recorded("build-app-image.sh")
        self.assertEqual([(call["args"][1], call["args"][2]) for call in builds], [("beszel_bridge", "0.21.1.4")])
        staged = [call["args"][1:] for call in self.recorded("git") if call["args"][0] == "add"]
        self.assertEqual(staged, [["beszel_bridge/config.yaml", "beszel_bridge/CHANGELOG.md"]])
        release = self.recorded("gh")[0]
        self.assertIn("bridge-v0.21.1.4", release["args"])
        self.assertIn("--latest=false", release["args"])
        self.assertIn("Beszel Bridge to version 0.21.1.4", release["notes"])
        self.assertIn("bridge-v0.21.1.4", (self.directory / "beszel_bridge/CHANGELOG.md").read_text())

    def test_full_rebuild_publishes_the_newer_bridge_at_its_own_version(self):
        directory = self.directory / "beszel_bridge"
        (directory / "config.yaml").write_text('version: "0.21.2.1"\n')
        result = plan(current_bridge="0.21.2.1", force=True)
        self.execute("Update selected app versions", result)
        self.execute("Publish images", result)
        builds = self.recorded("build-app-image.sh")
        self.assertEqual(len(builds), 7)
        self.assertEqual(next(call["args"][2] for call in builds if call["args"][1] == "beszel_bridge"), "0.21.2.1")
        self.assertEqual((directory / "CHANGELOG.md").read_text(), "original notes\n")

    def test_release_notes_describe_the_selected_apps(self):
        for scope in ("all", "all-except-bridge"):
            with self.subTest(scope=scope):
                self.execute("Create GitHub Release", plan(image_type=scope))
                release = self.recorded("gh")[-1]
                self.assertIn("v0.21.2", release["args"])
                self.assertIn("--verify-tag", release["args"])
                self.assertNotIn("--latest=false", release["args"])
                self.assertIn("Agent and Hub apps to version 0.21.2", release["notes"])
                self.assertEqual("Published Beszel Bridge" in release["notes"], scope == "all")
                self.assertEqual("Bridge was excluded" in release["notes"], scope == "all-except-bridge")

    def test_rebuilding_bridge_preserves_its_changelog(self):
        self.execute("Update selected app versions", plan(image_type="bridge"))
        self.assertEqual((self.directory / "beszel_bridge/CHANGELOG.md").read_text(), "original notes\n")


if __name__ == "__main__":
    unittest.main()

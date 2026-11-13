#!/usr/bin/env python3
"""Regression checks for container test runners, without Docker."""

import importlib.util
import json
from pathlib import Path
import subprocess
import shutil
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location(
    "agent_keys", Path(__file__).with_name("test-agent-keys.py")
)
AGENT_KEYS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(AGENT_KEYS)

BRIDGE_SPEC = importlib.util.spec_from_file_location(
    "bridge_e2e", Path(__file__).with_name("test-bridge-e2e.py")
)
BRIDGE_E2E = importlib.util.module_from_spec(BRIDGE_SPEC)
BRIDGE_SPEC.loader.exec_module(BRIDGE_E2E)


class ExitingAgent:
    """The final diagnostic appears when the container finishes exiting."""

    def __init__(self, exit_code=1, diagnostic="Failed to load public keys"):
        self.running = True
        self.exit_code = exit_code
        self.diagnostic = diagnostic
        self.removed = False

    def run(self, *command, **kwargs):
        output = b""
        error = b""
        operation = command[1]
        if operation == "run":
            output = b"test-container\n"
        elif operation == "inspect":
            self.running = False
            output = json.dumps({"Running": False, "ExitCode": self.exit_code}).encode()
        elif operation == "logs":
            output = b"Starting Beszel Agent...\n"
            if not self.running:
                error = self.diagnostic.encode()
        elif operation == "rm":
            self.removed = True
        else:
            raise AssertionError(f"Unexpected Docker command: {command}")
        return subprocess.CompletedProcess(command, 0, stdout=output, stderr=error)


class AgentExitTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="beszel-key-runner-")
        self.addCleanup(self.temporary.cleanup)
        self.work = Path(self.temporary.name)
        (self.work / "supervisor" / "addons" / "self" / "options").mkdir(parents=True)

    def check_exit(self, agent):
        with patch.object(AGENT_KEYS, "run", side_effect=agent.run):
            AGENT_KEYS.check_case(
                "test-image", "beszel_agent_intel", self.work, "test-network",
                {"key_file": "/ssl/invalid"}, error="Failed to load public keys",
            )

    def test_final_diagnostic_is_read_after_container_exit(self):
        agent = ExitingAgent()
        self.check_exit(agent)
        self.assertTrue(agent.removed)

    def test_unexpected_exit_is_still_rejected(self):
        agent = ExitingAgent(diagnostic="A different failure")
        with self.assertRaisesRegex(AssertionError, "Agent exited unexpectedly"):
            self.check_exit(agent)
        self.assertTrue(agent.removed)

    def test_zero_exit_code_cannot_satisfy_an_error_case(self):
        agent = ExitingAgent(exit_code=0)
        with self.assertRaisesRegex(AssertionError, "Agent exited unexpectedly"):
            self.check_exit(agent)
        self.assertTrue(agent.removed)


class BridgeFileTests(unittest.TestCase):
    def setUp(self):
        self.lab = BRIDGE_E2E.Lab(SimpleNamespace(keep=False))
        self.addCleanup(shutil.rmtree, self.lab.directory, ignore_errors=True)
        self.lab.bridge = "test-bridge"

    def test_private_state_is_read_without_host_file_access(self):
        with patch.object(Path, "read_text", side_effect=PermissionError("root-owned state")):
            with patch.object(self.lab, "docker", return_value='{"namespace":"test_namespace"}'):
                self.assertEqual(self.lab.bridge_state()["namespace"], "test_namespace")

    def test_root_owned_files_can_be_cleaned_up(self):
        private_file = self.lab.directory / "private"
        private_file.write_text("test-state")
        private_file.chmod(0o600)

        def remove_contents(*command):
            self.assertEqual(private_file.stat().st_mode & 0o777, 0o600)
            private_file.unlink()
            return ""

        with patch.object(BRIDGE_E2E.shutil, "rmtree", side_effect=PermissionError("root-owned directory")):
            with patch.object(self.lab, "docker", side_effect=remove_contents):
                self.lab.remove_test_directory()
        self.assertFalse(self.lab.directory.exists())


class BridgeTemperatureTests(unittest.TestCase):
    def setUp(self):
        self.lab = BRIDGE_E2E.Lab(SimpleNamespace(keep=False))
        self.addCleanup(shutil.rmtree, self.lab.directory, ignore_errors=True)
        self.lab.namespace = "test_namespace"
        self.lab.system_id = "test_system"

    def snapshot(self, *metrics):
        prefix = self.lab.namespace + "_" + self.lab.system_id + "_"
        return {"entities": [{"unique_id": prefix + metric} for metric in metrics]}

    def test_missing_sensors_allow_unrelated_temperature_entities(self):
        snapshot = self.snapshot("cpu_usage", "smart_disk_temperature", "gpus_card_temperature")
        snapshot["entities"].append({"unique_id": "another_system_temperature"})
        self.lab.assert_no_temperature_sensors(snapshot)

    def test_missing_sensors_reject_primary_temperature(self):
        with self.assertRaisesRegex(AssertionError, "without a source sensor"):
            self.lab.assert_no_temperature_sensors(self.snapshot("cpu_usage", "temperature"))

    def test_missing_sensors_reject_detailed_temperature(self):
        key = self.lab.component_key("temperatures", "cpu_package", "temperature")
        with self.assertRaisesRegex(AssertionError, "without a source sensor"):
            self.lab.assert_no_temperature_sensors(self.snapshot("cpu_usage", key))


if __name__ == "__main__":
    unittest.main()

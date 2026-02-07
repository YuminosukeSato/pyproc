"""Tests for CLI entry point."""

from __future__ import annotations

import importlib.util
import runpy
import sys

import pytest

from pyproc_worker import cli


def test_cli_executes_worker_script(tmp_path, monkeypatch) -> None:
    """CLI should import and execute the worker script."""
    script_path = tmp_path / "worker_script.py"
    script_path.write_text("VALUE = 'ok'\n")

    monkeypatch.setattr(sys, "argv", ["pyproc-worker", str(script_path)])

    cli.main()

    assert "worker" in sys.modules
    assert sys.modules["worker"].VALUE == "ok"


def test_cli_exits_when_spec_missing(monkeypatch) -> None:
    """CLI should exit with code 1 when import spec is missing."""

    def fake_spec_from_file_location(_name: str, _path: str):
        return None

    monkeypatch.setattr(importlib.util, "spec_from_file_location", fake_spec_from_file_location)
    monkeypatch.setattr(sys, "argv", ["pyproc-worker", "missing.py"])

    with pytest.raises(SystemExit) as exc:
        cli.main()

    assert exc.value.code == 1


def test_cli_module_main_invocation(tmp_path, monkeypatch) -> None:
    """Running the module as __main__ should invoke main()."""
    script_path = tmp_path / "worker_script.py"
    script_path.write_text("VALUE = 'ok'\n")

    monkeypatch.setattr(sys, "argv", ["pyproc-worker", str(script_path)])

    runpy.run_module("pyproc_worker.cli", run_name="__main__")

    assert "worker" in sys.modules
    assert sys.modules["worker"].VALUE == "ok"

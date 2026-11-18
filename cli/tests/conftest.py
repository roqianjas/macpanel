import json
import os
import sys
from pathlib import Path

import pytest

CLI_DIR = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(CLI_DIR))
os.environ["COLUMNS"] = "100"  # lebar tetap supaya teks bantuan argparse deterministik

LIVE = os.environ.get("MACPANEL_LIVE") == "1"


def pytest_configure(config):
    config.addinivalue_line("markers", "live: menyentuh mesin nyata (database, docker); butuh MACPANEL_LIVE=1")


def pytest_collection_modifyitems(config, items):
    if LIVE:
        return
    skip = pytest.mark.skip(reason="tes live: set MACPANEL_LIVE=1 untuk menjalankan")
    for it in items:
        if "live" in it.keywords:
            it.add_marker(skip)


@pytest.fixture
def golden():
    return json.loads((Path(__file__).parent / "golden" / "help.json").read_text())

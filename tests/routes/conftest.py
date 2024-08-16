from enum import Enum

import pytest
from pathlib import Path
from pytest.SubRequest
from src.config import TEMPLATES_DIR


class DirLevel(Enum):
    ROOT = "_default"
    TEAM = "atomverse"
    TEAM_LANGUAGE = "python"


@pytest.fixture(scope="function")
def temp_directory(request):
    TEMPLATES_DIR.mkdir(parents=True, exist_ok=True)
    temp_dir = f"temp_test_directory_{request.param}"

    if request.param == DirLevel.ROOT:
        (TEMPLATES_DIR / "_default").mkdir()
    elif request.param == DirLevel.TEAM:
        (TEMPLATES_DIR / DirLevel.TEAM.value).mkdir()
    elif request.param == DirLevel.TEAM_LANGUAGE:
        (TEMPLATES_DIR / DirLevel.TEAM.value / DirLevel.TEAM_LANGUAGE.value).mkdir()

    yield temp_dir

    TEMPLATES_DIR.rmdir()

"""Configuration constants and helpers for the VidDock helper."""

from __future__ import annotations

import os

PROTOCOL_VERSION = 1
MAX_FRAME_BYTES = 1024 * 1024
MAX_FILENAME_LENGTH = 80


def default_download_dir() -> str:
    """Return the default download directory path.

    The directory is not created; callers are responsible for ensuring it
    exists before writing files into it.
    """
    local_appdata = os.environ.get("LOCALAPPDATA")
    if not local_appdata:
        raise RuntimeError("LOCALAPPDATA environment variable is not set")
    return os.path.join(local_appdata, "VidDock", "downloads")

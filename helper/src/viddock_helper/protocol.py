"""Native Messaging framing layer for the VidDock helper.

Implements exactly one thing: the Chrome/Brave Native Messaging stdio
framing — a 4-byte little-endian unsigned length header followed by a
UTF-8 encoded JSON payload — plus a small typed error surface.

Deliberately NOT implemented here (later stages, see PLAN.md):
envelope validation (``type``/``id``/``command``), command dispatch,
and any I/O beyond ``read``/``write``/``flush`` on the given streams.

Error model, ``ProtocolError``:

- ``code``: machine-readable reason, see the ``CODE_*`` constants.
- ``message``: human-readable, safe to include in logs or return verbatim
  (never contains caller-controlled input such as URLs or file names).
- ``fatal``: whether the framed stream is in a state from which the next
  well-formed frame can no longer be located:

  * clean EOF (zero bytes on a fresh read of the 4-byte header) is
    reported with the ``CODE_EOF`` code and ``fatal=False`` — a normal
    browser disconnect. Clean EOF is only valid on that fresh header
    read (``clean_eof_on_zero=True``); it is never valid while a
    payload is in flight;
  * once a complete header has declared a payload length > 0, any EOF
    before the full payload is received is a fatal truncated-payload /
    incomplete-frame error (``CODE_INCOMPLETE_FRAME, fatal=True``);
  * an oversized *declared* frame raises with ``fatal=True`` **and the
    payload bytes are not read** (they are attacker-controlled and must
    not be drained into memory); the stream is then untrustworthy and the
    caller must close the connection;
  * partial header / truncated payload are protocol desyncs and raise
    ``fatal=True``;
  * invalid UTF-8 and malformed JSON are *per-frame* conditions: the
    frame was fully consumed, so the connection may be reused. They
    raise ``fatal=False`` (nonfatal by default).

Everything is standard library only.
"""

from __future__ import annotations

import json
import struct
from typing import Any, BinaryIO

from .config import MAX_FRAME_BYTES

__all__ = [
    "CODE_EOF",
    "CODE_INCOMPLETE_HEADER",
    "CODE_FRAME_SIZE",
    "CODE_INCOMPLETE_FRAME",
    "CODE_INVALID_UTF8",
    "CODE_MALFORMED_JSON",
    "ProtocolError",
    "read_frame",
    "write_frame",
]

_HEADER_STRUCT = struct.Struct("<I")
_HEADER_BYTES = _HEADER_STRUCT.size  # 4, little-endian unsigned int

#: Clean end of stream (zero header bytes read).
CODE_EOF = "EOF"
#: Header was partially available and the stream closed before 4 bytes.
CODE_INCOMPLETE_HEADER = "INCOMPLETE_HEADER"
#: Declared payload length exceeds MAX_FRAME_BYTES.
CODE_FRAME_SIZE = "FRAME_SIZE_EXCEEDED"
#: Payload was shorter than the declared length.
CODE_INCOMPLETE_FRAME = "INCOMPLETE_FRAME"
#: Payload bytes are not valid UTF-8.
CODE_INVALID_UTF8 = "INVALID_UTF8"
#: Payload decodes but is not valid JSON.
CODE_MALFORMED_JSON = "MALFORMED_JSON"


class ProtocolError(Exception):
    """Controlled protocol failure on a Native Messaging frame stream.

    Attributes:
        code: One of the ``CODE_*`` constants.
        message: Short, human-readable description (safe to log).
        fatal: True when the stream can no longer produce well-formed
            frames (caller should close the connection); False when the
            offending frame was fully consumed and the stream may be
            safely continued.
    """

    def __init__(self, code: str, message: str, fatal: bool) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code
        self.message = message
        self.fatal = fatal


def _raise_eof() -> ProtocolError:
    return ProtocolError(
        CODE_EOF,
        "clean EOF before any frame header byte",
        fatal=False,
    )


def _read_exact(stream: BinaryIO, count: int, incomplete_code: str,
                incomplete_desc: str, clean_eof_on_zero: bool = False) -> bytes:
    """Read exactly ``count`` bytes or raise a controlled error.

    Args:
        clean_eof_on_zero: When ``True``, only a *fresh* zero-byte read
            (zero bytes received so far) is treated as clean EOF —
            ``CODE_EOF`` with ``fatal=False``, meaning the caller was
            exactly at a frame boundary. This flag is used only for the
            fresh read of the 4-byte header. When ``False`` (the
            default), any zero-byte read before ``count`` bytes arrived
            — including a fresh one — raises
            ``ProtocolError(incomplete_code, fatal=True)``: once a
            complete header has declared a payload length > 0 the frame
            is truncated and the stream is desynchronized, no matter how
            many payload bytes were already received.

    Raises:
        ProtocolError(CODE_EOF, fatal=False): a fresh read of zero bytes
            with ``clean_eof_on_zero=True`` (clean end of stream at a
            frame boundary).
        ProtocolError(incomplete_code, fatal=True): the stream closed
            before ``count`` bytes were received (zero bytes allowed by
            this flag are the exception; see above).
    """
    chunks: list[bytes] = []
    received = 0
    while received < count:
        chunk = stream.read(count - received)
        if not chunk:
            if received == 0 and clean_eof_on_zero:
                # Zero bytes on a fresh *header* read: clean EOF.
                # Never valid for payload reads (clean_eof_on_zero=False),
                # where a complete header already declared count > 0.
                raise _raise_eof()
            raise ProtocolError(
                incomplete_code,
                f"{incomplete_desc}: {received} of {count} "
                "bytes received",
                fatal=True,
            )
        chunks.append(chunk)
        received += len(chunk)
    return b"".join(chunks)


def read_frame(stream: BinaryIO) -> Any:
    """Read one Native Messaging frame from ``stream``.

    Returns:
        The parsed JSON payload (parsed exactly once; any JSON value
        type). Envelope/type checking is a later stage's job.

    Raises:
        ProtocolError: one of the ``CODE_*`` conditions; see the module
            docstring for fatality semantics. A ``fatal=False`` error
            with code != ``CODE_EOF`` means exactly one frame was fully
            consumed and the stream remains readable; ``CODE_EOF`` and
            every ``fatal=True`` code mean the caller must stop reading.
    """
    header = _read_exact(
        stream,
        _HEADER_BYTES,
        CODE_INCOMPLETE_HEADER,
        "frame header",
        clean_eof_on_zero=True,
    )
    (length,) = _HEADER_STRUCT.unpack(header)

    if length > MAX_FRAME_BYTES:
        # Fatal, and deliberate: the declared payload is up to ~4 GiB of
        # attacker-controlled bytes. Draining it into memory (or discarding
        # it byte by byte) would amplify a short malicious message into a
        # memory/CPU DoS, so we surface the error *without reading a single
        # payload byte*. The stream position is now undefined, which is why
        # the condition is fatal: the caller must close the connection.
        raise ProtocolError(
            CODE_FRAME_SIZE,
            f"declared frame length {length} exceeds limit "
            f"{MAX_FRAME_BYTES}; payload intentionally not read",
            fatal=True,
        )

    # Default clean_eof_on_zero=False: the complete header above already
    # declared the payload length, so any EOF before `length` bytes
    # arrived (0, partial, or otherwise) is a truncated payload.
    payload = _read_exact(
        stream,
        length,
        CODE_INCOMPLETE_FRAME,
        "frame payload",
    )
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError:
        # Frame fully consumed above, so the stream is still in sync.
        raise ProtocolError(
            CODE_INVALID_UTF8,
            "payload is not valid UTF-8",
            fatal=False,
        ) from None

    try:
        value: Any = json.loads(text)
    except json.JSONDecodeError:
        # Same: the frame was fully consumed, so this is recoverable.
        raise ProtocolError(
            CODE_MALFORMED_JSON,
            "payload is not valid JSON",
            fatal=False,
        ) from None
    return value


def write_frame(stream: BinaryIO, value: Any) -> None:
    """Write one framed JSON response to ``stream`` and flush it.

    Args:
        stream: A binary stream with ``write`` and ``flush`` (e.g.
            ``sys.stdout.buffer``).
        value: Any JSON-serializable value.

    Raises:
        ProtocolError: ``CODE_FRAME_SIZE`` (nonfatal) if the serialized
            payload would exceed ``MAX_FRAME_BYTES``, or
            ``CODE_MALFORMED_JSON`` (nonfatal) if ``value`` is not
            JSON-serializable at all. In both cases no frame bytes have
            been written (the header is written only after the payload
            bytes are known), so the stream remains fully in sync in
            either case.

    Note:
        The header and payload are written in a single buffered chunk
        followed by one flush, so a single logical frame is a single
        ``flush``-delivered unit on stdio.
    """
    try:
        payload = json.dumps(
            value, separators=(",", ":"), ensure_ascii=True,
            allow_nan=False,
        ).encode("ascii")
    except (TypeError, ValueError):
        raise ProtocolError(
            CODE_MALFORMED_JSON,
            "response value is not JSON-serializable",
            fatal=False,
        ) from None

    if len(payload) > MAX_FRAME_BYTES:
        raise ProtocolError(
            CODE_FRAME_SIZE,
            f"response payload length {len(payload)} exceeds limit "
            f"{MAX_FRAME_BYTES}",
            fatal=False,
        ) from None

    frame = _HEADER_STRUCT.pack(len(payload)) + payload
    stream.write(frame)
    stream.flush()

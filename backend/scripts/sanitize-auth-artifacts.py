#!/usr/bin/env python3
"""Redact structured and textual credentials; discard raw packet captures.

Auth BDD exports static step names and status only. This also protects diagnostic
files from other clients before upload. Packet captures cannot be safely redacted.
"""
import json
import os
from pathlib import Path
import re
import sys

KEY = re.compile(r'password|token|authorization|(^|_)code($|_)|hmac|secret|digest', re.I)
JWT = re.compile(r'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+')
OTP = re.compile(r'(?<![A-Za-z0-9.])[0-9]{6}(?![A-Za-z0-9.])')
SENSITIVE = [v for k, v in os.environ.items() if KEY.search(k) and len(v) >= 8]

def scrub(value):
    if isinstance(value, dict):
        return {k: '[REDACTED]' if KEY.search(k) else scrub(v) for k, v in value.items()}
    if isinstance(value, list):
        return [scrub(v) for v in value]
    if isinstance(value, str):
        for secret in SENSITIVE:
            value = value.replace(secret, '[REDACTED]')
        value = JWT.sub('[REDACTED]', value)
        value = re.sub(r'(?i)(Bearer\s+)[^\s"<>]+', r'\1[REDACTED]', value)
        value = re.sub(r'(?i)("(?:password|current_password|new_password|code|token)"\s*:\s*")[^"]*', r'\1[REDACTED]', value)
        return OTP.sub('[REDACTED]', value)
    return value

if __name__ == '__main__':
    for arg in sys.argv[1:]:
        root = Path(arg)
        for path in root.rglob('*'):
            if not path.is_file():
                continue
            if path.suffix in ('.pcap', '.pcapng'):
                path.unlink()
                continue
            if path.suffix not in ('.json', '.xml', '.txt', '.log', '.body', '.headers'):
                continue
            raw = path.read_text(errors='replace')
            try:
                cleaned = json.dumps(scrub(json.loads(raw)), ensure_ascii=False)
            except ValueError:
                cleaned = scrub(raw)
            path.write_text(cleaned)

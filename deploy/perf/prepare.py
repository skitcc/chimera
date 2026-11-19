#!/usr/bin/env python3
"""Explicit deployment/configuration step, separate from measurement."""
import os
import fcntl
from pathlib import Path
import subprocess
import sys
from runner import Runner, ROOT


def main():
    lock = (ROOT / '.perf-run.lock').open('a')
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        raise SystemExit('cannot deploy while a perf run is active')
    runner = Runner()
    overlay = ROOT / 'deploy/k8s/overlays' / ('minikube' if os.getenv('PERF_RESTORE') == '1' else 'perf')
    try:
        print(f'Apply {overlay}; databases, PVCs and Kafka data are preserved', flush=True)
        result = subprocess.run(runner.kubectl + ['apply', '-k', str(overlay)], timeout=90)
        if result.returncode:
            return result.returncode
        if os.getenv('PERF_RESTORE') != '1':
            runner.preflight()
        return 0
    except Exception as exc:
        print(f'Preparation failed: {exc}', file=sys.stderr)
        runner.diagnostics()
        return 1


if __name__ == '__main__':
    sys.exit(main())

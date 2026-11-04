"""Inert documented-command fixture using the real agent argument parser."""

from pathlib import Path


INSTALLER = Path(__file__).resolve().parents[2] / "scripts/install.sh"


def credential_path_parser_fixture():
    # Keep the actual source through argument parsing, but never include the
    # subsequent saved-state, credential-reader or installation operations.
    prefix, boundary, _ = INSTALLER.read_text().partition(
        'if [[ "$RETARGET_ONLY" == "true" && -z "$PULSE_URL" ]]; then'
    )
    if not boundary:
        raise AssertionError("agent parser boundary missing; refusing to build fixture")
    return '#!/usr/bin/env bash\nDOCS_INSTALLER_ARGS=("$@")\n' + prefix + r'''
python3 - "$TOKEN_FILE_PATH" "${DOCS_INSTALLER_ARGS[@]}" <<'PY'
import json, os, stat, sys
from pathlib import Path
parsed_path = sys.argv[1]
args = sys.argv[2:]
assert args.count('--token-file') == 1
assert parsed_path == args[args.index('--token-file') + 1], "token path changed during parsing"
path = Path(parsed_path)
assert stat.S_IMODE(path.stat().st_mode) == 0o600
assert path.read_text().strip() == os.environ['EXPECTED_TOKEN']
assert '--token' not in args
assert os.environ['EXPECTED_TOKEN'] not in ' '.join(args)
Path(os.environ['INSTALL_RECEIPT']).write_text(json.dumps(args))
PY
}
main "$@"
'''

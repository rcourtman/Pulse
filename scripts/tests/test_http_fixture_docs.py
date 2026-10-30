#!/usr/bin/env python3
"""Check the owned lifecycle of documentation HTTP/TLS fixtures.

Run in pulse-worker-source-proof: listeners are guest-local and credentials,
certificates and installers are synthetic. The consumer suites test requests;
these controls test idle teardown, exception propagation and listener isolation.
"""

from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch

import test_api_auth_docs as api
import test_authentication_diagnostic_docs as authentication
import test_pbs_docs as pbs
import test_scheduler_polling_docs as scheduler
import test_signed_installer_docs as installer
import test_truenas_docs as truenas


REAL_THREAD = threading.Thread


class FixtureFailure(Exception):
    pass


class HTTPFixtureLifecycleTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory(prefix="pulse-http-lifecycle-")
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.cert, cls.key = pbs.certificate(Path(cls.temporary.name), "server")

    def fixtures(self):
        return (
            ("api", lambda _: api.recording_server(), 1),
            ("proxy", lambda _: authentication.proxy_server(403), 1),
            ("pbs-tls", lambda _: pbs.server(self.cert, self.key), 1),
            ("scheduler", lambda _: scheduler.server(scheduler.snapshot()), 1),
            ("signed-install-tls", lambda root: installer.download_servers(root, "success"), 2),
            ("truenas", lambda _: truenas.server(), 1),
        )

    def check_lifecycle(self, factory, expected_listeners, *, exceptional):
        owned = []

        def capture_thread(*args, **kwargs):
            thread = REAL_THREAD(*args, **kwargs)
            owned.append((thread, kwargs["target"].__self__, kwargs))
            return thread

        def exercise(root):
            with factory(root):
                self.assertEqual(len(owned), expected_listeners)
                self.assertTrue(all(thread.is_alive() for thread, _, _ in owned))
                self.assertEqual(len({server.server_address for _, server, _ in owned}),
                                 expected_listeners)
                if exceptional:
                    raise FixtureFailure("synthetic consumer failure")

        with tempfile.TemporaryDirectory() as temporary, \
                patch.object(threading, "Thread", side_effect=capture_thread):
            if exceptional:
                with self.assertRaisesRegex(FixtureFailure, "synthetic consumer failure"):
                    exercise(Path(temporary))
            else:
                exercise(Path(temporary))

        # Observe real cleanup before judging scheduling. A default-poll
        # regression must not strand its listener just to fail this control.
        self.assertEqual(len(owned), expected_listeners)
        for thread, server, kwargs in owned:
            self.assertFalse(thread.is_alive(), "fixture thread survived teardown")
            self.assertEqual(server.socket.fileno(), -1, "fixture listener remained open")
            self.assertEqual(server.server_address[0], "127.0.0.1")
            self.assertTrue(kwargs["daemon"])
            self.assertEqual(kwargs.get("kwargs"), {"poll_interval": 0.05},
                             "idle teardown must not wait for the default 500 ms poll")

    def test_idle_fixtures_close_every_owned_listener_and_thread(self):
        for name, factory, expected in self.fixtures():
            with self.subTest(fixture=name):
                self.check_lifecycle(factory, expected, exceptional=False)

    def test_consumer_failure_propagates_after_every_listener_is_closed(self):
        for name, factory, expected in self.fixtures():
            with self.subTest(fixture=name):
                self.check_lifecycle(factory, expected, exceptional=True)


if __name__ == "__main__":
    unittest.main()

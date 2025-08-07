#!/usr/bin/env python3
"""
Mock GitHub API server for testing Pulse updates locally
"""

import json
import os
from http.server import HTTPServer, SimpleHTTPRequestHandler
from datetime import datetime
import sys

class MockGitHubHandler(SimpleHTTPRequestHandler):
    def do_GET(self):
        # Mock the GitHub releases API endpoint
        if self.path == '/repos/rcourtman/Pulse/releases/latest':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            
            # Mock release data - customize version here for testing
            release_data = {
                "tag_name": "v4.0.99",  # Fake higher version for testing
                "name": "v4.0.99 - Test Release",
                "body": "## Test Release\n\nThis is a mock release for testing the update functionality.\n\n### Features\n- Test update button\n- Test update process\n\n### Notes\nThis is not a real release!",
                "published_at": datetime.now().isoformat() + "Z",
                "prerelease": False,
                "assets": [
                    {
                        "name": "pulse-v4.0.99-linux-amd64.tar.gz",
                        "browser_download_url": f"http://{self.headers['Host']}/files/pulse-v4.0.99-linux-amd64.tar.gz"
                    },
                    {
                        "name": "pulse-v4.0.99-linux-arm64.tar.gz", 
                        "browser_download_url": f"http://{self.headers['Host']}/files/pulse-v4.0.99-linux-arm64.tar.gz"
                    }
                ]
            }
            self.wfile.write(json.dumps(release_data).encode())
            
        # Mock the file download endpoints
        elif self.path.startswith('/files/'):
            # Serve actual release files from the files/ directory
            file_path = self.path[7:]  # Remove '/files/' prefix
            local_file = os.path.join('files', file_path)
            
            if os.path.exists(local_file):
                self.send_response(200)
                self.send_header('Content-Type', 'application/gzip')
                self.end_headers()
                with open(local_file, 'rb') as f:
                    self.wfile.write(f.read())
            else:
                self.send_response(404)
                self.end_headers()
                self.wfile.write(b"File not found")
        else:
            self.send_response(404)
            self.end_headers()

def run_server(port=8888):
    server_address = ('', port)
    httpd = HTTPServer(server_address, MockGitHubHandler)
    print(f"Mock GitHub API server running on port {port}")
    print(f"Test with: curl http://localhost:{port}/repos/rcourtman/Pulse/releases/latest")
    httpd.serve_forever()

if __name__ == '__main__':
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8888
    run_server(port)
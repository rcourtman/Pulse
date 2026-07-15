package tools

import "testing"

// This file raises branch coverage for the curl/wget read-only classifier
// helpers in tools_query.go:
//   - isHTTPMutationMethod
//   - shortCurlOptionCluster
//   - curlHasHTTPURL
//   - isWgetWriteOrMutationOption
//   - isCurlMutationOrFileOutput
//
// Each function is exercised through table-driven cases that target the
// uncovered switch arms, guard clauses, and early/return-false paths.

// TestIsHTTPMutationMethod_BranchCov covers every case arm of
// isHTTPMutationMethod including the trimming, lowercasing and the default
// (non-mutation) arm.
func TestIsHTTPMutationMethod_BranchCov(t *testing.T) {
	tests := []struct {
		name   string
		method string
		want   bool
	}{
		{name: "post_lower", method: "post", want: true},
		{name: "put_lower", method: "put", want: true},
		{name: "patch_lower", method: "patch", want: true},
		{name: "delete_lower", method: "delete", want: true},
		{name: "post_upper", method: "POST", want: true},
		{name: "post_mixed_case", method: "PoSt", want: true},
		{name: "delete_upper", method: "DELETE", want: true},
		{name: "post_padded", method: "  POST  ", want: true},
		{name: "patch_tab", method: "\tpatch\n", want: true},
		{name: "get_is_not_mutation", method: "GET", want: false},
		{name: "get_lower", method: "get", want: false},
		{name: "head", method: "HEAD", want: false},
		{name: "options", method: "OPTIONS", want: false},
		{name: "connect", method: "CONNECT", want: false},
		{name: "trace", method: "TRACE", want: false},
		{name: "empty", method: "", want: false},
		{name: "unknown", method: "BOGUS", want: false},
		{name: "connect_lower_padded", method: "  connect  ", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isHTTPMutationMethod(tc.method)
			if got != tc.want {
				t.Fatalf("isHTTPMutationMethod(%q) = %v, want %v", tc.method, got, tc.want)
			}
		})
	}
}

// TestShortCurlOptionCluster_BranchCov covers every return path of
// shortCurlOptionCluster: non-option tokens, long options, too-short tokens,
// the -X/-d exclusions and the normal short-option cluster return.
func TestShortCurlOptionCluster_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "no_dash_prefix", token: "output", want: ""},
		{name: "bare_word", token: "http://x", want: ""},
		{name: "empty", token: "", want: ""},
		{name: "single_dash", token: "-", want: ""},
		{name: "short_o_len2", token: "-o", want: ""},
		{name: "short_L_len2", token: "-L", want: ""},
		{name: "short_K_len2", token: "-K", want: ""},
		{name: "short_F_len2", token: "-F", want: ""},
		{name: "long_output", token: "--output", want: ""},
		{name: "long_request_equals", token: "--request=POST", want: ""},
		{name: "long_data_raw", token: "--data-raw", want: ""},
		{name: "X_attached_excluded", token: "-XPOST", want: ""},
		{name: "X_attached_lower", token: "-xpost", want: "xpost"},
		{name: "d_attached_excluded", token: "-ddata", want: ""},
		{name: "cluster_vL", token: "-vL", want: "vL"},
		{name: "cluster_sO", token: "-sO", want: "sO"},
		{name: "cluster_sK", token: "-sK", want: "sK"},
		{name: "cluster_sF", token: "-sF", want: "sF"},
		{name: "cluster_sc", token: "-sc", want: "sc"},
		{name: "cluster_silent", token: "-s", want: ""}, // len 2 < 3
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shortCurlOptionCluster(tc.token)
			if got != tc.want {
				t.Fatalf("shortCurlOptionCluster(%q) = %q, want %q", tc.token, got, tc.want)
			}
		})
	}
}

// TestCurlHasHTTPURL_BranchCov covers every arm of curlHasHTTPURL: direct
// http(s):// prefixes, the --url space-separated form (matching and
// non-matching next token), the --url= attached form, the --url-at-end guard,
// quoted URLs and the no-match default.
func TestCurlHasHTTPURL_BranchCov(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   bool
	}{
		{name: "empty", fields: []string{}, want: false},
		{name: "http_direct", fields: []string{"http://example.com"}, want: true},
		{name: "https_direct", fields: []string{"https://example.com"}, want: true},
		{name: "http_uppercase", fields: []string{"HTTP://EXAMPLE.COM"}, want: true},
		{name: "https_mixed_case", fields: []string{"HtTpS://example.com"}, want: true},
		{name: "http_quoted", fields: []string{`"http://example.com"`}, want: true},
		{name: "http_single_quoted", fields: []string{"'https://example.com'"}, want: true},
		{name: "ftp_not_http", fields: []string{"ftp://example.com"}, want: false},
		{name: "ws_not_http", fields: []string{"ws://example.com"}, want: false},
		{name: "bare_host", fields: []string{"example.com"}, want: false},
		{name: "http_after_option", fields: []string{"-s", "http://example.com"}, want: true},
		{name: "url_space_http", fields: []string{"--url", "http://example.com"}, want: true},
		{name: "url_space_https", fields: []string{"--url", "https://example.com"}, want: true},
		{name: "url_space_https_upper", fields: []string{"--url", "HTTPS://example.com"}, want: true},
		{name: "url_space_https_quoted", fields: []string{"--url", `"https://example.com"`}, want: true},
		{name: "url_space_ftp", fields: []string{"--url", "ftp://example.com"}, want: false},
		{name: "url_space_bare", fields: []string{"--url", "example.com"}, want: false},
		{name: "url_at_end_no_next", fields: []string{"--url"}, want: false},
		{name: "url_equals_http", fields: []string{"--url=http://example.com"}, want: true},
		{name: "url_equals_https", fields: []string{"--url=https://example.com"}, want: true},
		{name: "url_equals_https_upper", fields: []string{"--url=HTTPS://EXAMPLE.COM"}, want: true},
		{name: "url_equals_ftp", fields: []string{"--url=ftp://example.com"}, want: false},
		{name: "url_equals_bare", fields: []string{"--url=example.com"}, want: false},
		{name: "options_only_no_url", fields: []string{"-s", "-L", "-o", "out"}, want: false},
		{name: "url_space_http_after_opts", fields: []string{"-s", "--url", "http://x"}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := curlHasHTTPURL(tc.fields)
			if got != tc.want {
				t.Fatalf("curlHasHTTPURL(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}

// TestIsWgetWriteOrMutationOption_BranchCov covers every switch arm of
// isWgetWriteOrMutationOption. The test reproduces the real caller's contract
// by passing tokenLower = strings.ToLower(token).
func TestIsWgetWriteOrMutationOption_BranchCov(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		// Case 1: -O / --output-document
		{name: "O_exact", token: "-O", want: true},
		{name: "O_attached", token: "-Ofile", want: true},
		{name: "output_document_long", token: "--output-document", want: true},
		{name: "output_document_equals", token: "--output-document=-", want: true},

		// Case 2: -o / -a / --output-file / --append-output
		{name: "o_exact", token: "-o", want: true},
		{name: "o_attached", token: "-olog", want: true},
		{name: "a_exact", token: "-a", want: true},
		{name: "a_attached", token: "-alog", want: true},
		{name: "output_file_long", token: "--output-file", want: true},
		{name: "output_file_equals", token: "--output-file=err.log", want: true},
		{name: "append_output_long", token: "--append-output", want: true},
		{name: "append_output_equals", token: "--append-output=x.log", want: true},

		// Case 3: -P / --directory-prefix
		{name: "P_exact", token: "-P", want: true},
		{name: "P_attached", token: "-Pdir", want: true},
		{name: "directory_prefix_long", token: "--directory-prefix", want: true},
		{name: "directory_prefix_equals", token: "--directory-prefix=/tmp", want: true},

		// Case 4: --post-data / --post-file / --body-data / --body-file
		{name: "post_data_long", token: "--post-data", want: true},
		{name: "post_data_equals", token: "--post-data=a=1", want: true},
		{name: "post_file_long", token: "--post-file", want: true},
		{name: "post_file_equals", token: "--post-file=/tmp/body", want: true},
		{name: "body_data_long", token: "--body-data", want: true},
		{name: "body_data_equals", token: "--body-data=payload", want: true},
		{name: "body_file_long", token: "--body-file", want: true},
		{name: "body_file_equals", token: "--body-file=/tmp/body", want: true},

		// Case 5: --method
		{name: "method_long", token: "--method", want: true},
		{name: "method_equals", token: "--method=PUT", want: true},

		// Case 6: --warc-file
		{name: "warc_file_long", token: "--warc-file", want: true},
		{name: "warc_file_equals", token: "--warc-file=dump.warc", want: true},

		// Default arm (not a write/mutation option).
		{name: "spider_is_not", token: "--spider", want: false},
		{name: "quiet_is_not", token: "-q", want: false},
		{name: "verbose_is_not", token: "-v", want: false},
		{name: "url_is_not", token: "http://example.com", want: false},
		{name: "empty_is_not", token: "", want: false},
		{name: "random_long_is_not", token: "--no-check-certificate", want: false},
		{name: "random_long_proxy_is_not", token: "--proxy", want: false},
		{name: "uppercase_O_only_matches_O", token: "-OFILE", want: true}, // HasPrefix -O capital
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenLower := toLowerLocal(tc.token)
			got := isWgetWriteOrMutationOption(tc.token, tokenLower)
			if got != tc.want {
				t.Fatalf("isWgetWriteOrMutationOption(%q, %q) = %v, want %v",
					tc.token, tokenLower, got, tc.want)
			}
		})
	}
}

// toLowerLocal mirrors strings.ToLower without adding an extra import, keeping
// the test file's import list to just "testing" as is conventional for the
// pure helpers. It is used solely to reproduce the caller contract for
// isWgetWriteOrMutationOption.
func toLowerLocal(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// TestIsCurlMutationOrFileOutput_BranchCov covers the guard clause and every
// switch arm of isCurlMutationOrFileOutput, including the -X non-mutation
// fall-through paths that continue scanning the remaining tokens.
func TestIsCurlMutationOrFileOutput_BranchCov(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Guard: no curl token present.
		{name: "guard_no_curl", command: "echo hello", want: false},
		{name: "guard_other_cmd", command: "wget http://x", want: false},
		{name: "guard_substring_not_token", command: "notcurl http://x", want: false},
		{name: "guard_empty", command: "", want: false},

		// No mutation / file-output option: read-only HTTP request.
		{name: "plain_http_no_opts", command: "curl http://example.com", want: false},
		{name: "silent_location_no_mutation", command: "curl -s -L http://example.com", want: false},
		{name: "only_headers_no_mutation", command: "curl -I http://example.com", want: false},

		// -X / --request separated forms.
		{name: "X_post_separated", command: "curl -X POST http://x", want: true},
		{name: "X_put_separated", command: "curl -X PUT http://x", want: true},
		{name: "X_patch_separated", command: "curl -X PATCH http://x", want: true},
		{name: "X_delete_separated", command: "curl -X DELETE http://x", want: true},
		{name: "X_get_nonmutation_falls_through", command: "curl -X GET http://x", want: false},
		{name: "X_get_then_data_continues", command: "curl -X GET -d a=1 http://x", want: true},
		{name: "X_at_end_no_next", command: "curl -X", want: false},
		{name: "request_long_post_separated", command: "curl --request POST http://x", want: true},
		{name: "request_long_get_nonmutation", command: "curl --request GET http://x", want: false},

		// -X / --request attached (= / glued) forms.
		{name: "X_post_attached", command: "curl -XPOST http://x", want: true},
		{name: "X_get_attached_nonmutation", command: "curl -XGET http://x", want: false},
		{name: "request_equals_post", command: "curl --request=POST http://x", want: true},
		{name: "request_equals_get_nonmutation", command: "curl --request=GET http://x", want: false},

		// -d / --data* / --json (space-separated) forms.
		{name: "d_separated", command: "curl -d a=1 http://x", want: true},
		{name: "d_attached", command: "curl -da=1 http://x", want: true},
		{name: "data_long", command: "curl --data a=1 http://x", want: true},
		{name: "data_raw_long", command: "curl --data-raw a=1 http://x", want: true},
		{name: "data_binary_long", command: "curl --data-binary @file http://x", want: true},
		{name: "data_urlencode_long", command: "curl --data-urlencode a=1 http://x", want: true},
		{name: "json_long", command: "curl --json {..} http://x", want: true},

		// -F / --form forms.
		{name: "F_separated", command: "curl -F field=val http://x", want: true},
		{name: "F_attached", command: "curl -Ffield=val http://x", want: true},
		{name: "form_long", command: "curl --form field=val http://x", want: true},
		{name: "form_string_long", command: "curl --form-string field=val http://x", want: true},

		// -T / --upload forms.
		{name: "T_separated", command: "curl -T file http://x", want: true},
		{name: "T_attached", command: "curl -Tfile http://x", want: true},
		{name: "upload_file_long", command: "curl --upload-file file http://x", want: true},

		// -o / -O / --output* / --remote-name / cluster with o|O.
		{name: "o_separated", command: "curl -o out http://x", want: true},
		{name: "o_attached", command: "curl -oout http://x", want: true},
		{name: "O_capital", command: "curl -O http://x", want: true},
		{name: "cluster_with_O", command: "curl -sO http://x", want: true},
		{name: "cluster_with_o", command: "curl -vo http://x", want: true},
		{name: "output_long", command: "curl --output out http://x", want: true},
		{name: "output_equals", command: "curl --output=out http://x", want: true},
		{name: "output_dir_long", command: "curl --output-dir d http://x", want: true},
		{name: "output_dir_equals", command: "curl --output-dir=d http://x", want: true},
		{name: "remote_name_long", command: "curl --remote-name http://x", want: true},
		{name: "remote_header_name_long", command: "curl --remote-header-name http://x", want: true},

		// -c / --cookie-jar forms.
		{name: "c_separated", command: "curl -c jar http://x", want: true},
		{name: "c_attached", command: "curl -cjar http://x", want: true},
		{name: "cookie_jar_long", command: "curl --cookie-jar jar http://x", want: true},
		{name: "cookie_jar_equals", command: "curl --cookie-jar=jar http://x", want: true},

		// -K / --dump-header / --config / cluster with K.
		{name: "K_separated", command: "curl -K cfg http://x", want: true},
		{name: "cluster_with_K", command: "curl -sK http://x", want: true},
		{name: "dump_header_long", command: "curl --dump-header h http://x", want: true},
		{name: "dump_header_equals", command: "curl --dump-header=h http://x", want: true},
		{name: "config_long", command: "curl --config cfg http://x", want: true},
		{name: "config_equals", command: "curl --config=cfg http://x", want: true},

		// Cluster with F | T | c (last switch arm).
		{name: "cluster_with_F", command: "curl -sF http://x", want: true},
		{name: "cluster_with_T", command: "curl -sT http://x", want: true},
		{name: "cluster_with_c", command: "curl -sc http://x", want: true},

		// Quoted tokens are trimmed before matching.
		{name: "quoted_o_separated", command: `curl -o "out" http://x`, want: true},
		{name: "quoted_X_post", command: `curl -X "POST" http://x`, want: true},

		// curl reachable via a filesystem path (commandTokenBase strips path).
		{name: "curl_path_X_post", command: "/usr/bin/curl -X POST http://x", want: true},
		{name: "curl_path_no_mutation", command: "/usr/local/bin/curl http://x", want: false},

		// Quoted curl binary still recognised after quote trimming.
		{name: "quoted_curl_binary_X_put", command: `"curl" -X PUT http://x`, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isCurlMutationOrFileOutput(tc.command)
			if got != tc.want {
				t.Fatalf("isCurlMutationOrFileOutput(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

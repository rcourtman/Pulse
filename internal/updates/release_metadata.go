package updates

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// GitHub release metadata embeds every uploaded asset. A 6.4.x release carries
// hundreds of assets (~600 KB of JSON each) and the default 30-release page is
// several megabytes, so buffering the whole response behind a small bound made
// the update check fail for every install (#1881, #2282). The decoders below
// stream the response token by token and retain only the fields the updater
// consumes, so memory stays proportional to what is kept rather than to the
// response size. maxReleaseMetadataBytes still bounds the bytes read.

// decodeReleaseList streams a GitHub release-list response, keeping only the
// assets that isRuntimeReleaseAssetName accepts.
func decodeReleaseList(resp *http.Response) ([]ReleaseInfo, error) {
	if err := securityutil.LimitResponseBody(resp, maxReleaseMetadataBytes); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	if err := expectJSONDelim(dec, '['); err != nil {
		return nil, err
	}
	var releases []ReleaseInfo
	for dec.More() {
		release, err := decodeReleaseObject(dec, isRuntimeReleaseAssetName)
		if err != nil {
			return nil, err
		}
		releases = append(releases, release)
	}
	if err := expectJSONDelim(dec, ']'); err != nil {
		return nil, err
	}
	if err := expectJSONEnd(dec); err != nil {
		return nil, err
	}
	return releases, nil
}

// decodeSingleRelease streams one GitHub release object without retaining any
// assets; callers that only need release notes and dates never read them.
func decodeSingleRelease(resp *http.Response) (ReleaseInfo, error) {
	if err := securityutil.LimitResponseBody(resp, maxReleaseMetadataBytes); err != nil {
		return ReleaseInfo{}, err
	}
	dec := json.NewDecoder(resp.Body)
	release, err := decodeReleaseObject(dec, func(string) bool { return false })
	if err != nil {
		return ReleaseInfo{}, err
	}
	if err := expectJSONEnd(dec); err != nil {
		return ReleaseInfo{}, err
	}
	return release, nil
}

// isRuntimeReleaseAssetName matches the Pulse server archives the update check
// can offer: the exact runtime asset and the any-linux-tarball fallback.
func isRuntimeReleaseAssetName(name string) bool {
	return strings.HasPrefix(name, "pulse-") &&
		strings.Contains(name, "linux") &&
		strings.HasSuffix(name, ".tar.gz")
}

func decodeReleaseObject(dec *json.Decoder, keepAsset func(string) bool) (ReleaseInfo, error) {
	var release ReleaseInfo
	if err := expectJSONDelim(dec, '{'); err != nil {
		return release, err
	}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return release, err
		}
		key, ok := token.(string)
		if !ok {
			return release, fmt.Errorf("unexpected release field token %v", token)
		}
		switch key {
		case "tag_name":
			err = dec.Decode(&release.TagName)
		case "name":
			err = dec.Decode(&release.Name)
		case "body":
			err = dec.Decode(&release.Body)
		case "prerelease":
			err = dec.Decode(&release.Prerelease)
		case "draft":
			err = dec.Decode(&release.Draft)
		case "published_at":
			err = dec.Decode(&release.PublishedAt)
		case "assets":
			release.Assets, err = decodeReleaseAssets(dec, keepAsset)
		default:
			err = skipJSONValue(dec)
		}
		if err != nil {
			return release, fmt.Errorf("decode release field %q: %w", key, err)
		}
	}
	return release, expectJSONDelim(dec, '}')
}

func decodeReleaseAssets(dec *json.Decoder, keepAsset func(string) bool) ([]ReleaseAsset, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("expected assets array, got %v", token)
	}
	var assets []ReleaseAsset
	for dec.More() {
		var asset ReleaseAsset
		if err := dec.Decode(&asset); err != nil {
			return nil, err
		}
		if keepAsset(asset.Name) {
			assets = append(assets, asset)
		}
	}
	if err := expectJSONDelim(dec, ']'); err != nil {
		return nil, err
	}
	return assets, nil
}

// skipJSONValue consumes the next value token by token so large fields that
// the updater ignores are never buffered as a whole.
func skipJSONValue(dec *json.Decoder) error {
	depth := 0
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
		if depth == 0 {
			return nil
		}
	}
}

func expectJSONDelim(dec *json.Decoder, want json.Delim) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != want {
		return fmt.Errorf("expected %q in release metadata, got %v", want, token)
	}
	return nil
}

func expectJSONEnd(dec *json.Decoder) error {
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected data after release metadata")
	}
	return nil
}

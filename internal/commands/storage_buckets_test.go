package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func setupStorageHTTP(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	t.Cleanup(setupTest(t, newMockAPI()))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	apiClient = client.New(server.URL, "test-key-123")
}

func storageResponse(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data}))
}

func runStorageCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newStorageCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func TestStorageBucketWorkflow(t *testing.T) {
	const bucketID = "bucket-123"
	const bucketName = "media"
	const folder = "images & #/"
	const objectKey = folder + "file.txt"
	const contents = "Dilmune bucket round trip.\n"
	var mu sync.Mutex
	var stored []byte
	var confirmed bool
	var deleted bool

	setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/blob" {
			assert.Equal(t, "Bearer test-key-123", r.Header.Get("Authorization"))
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/buckets":
			storageResponse(t, w, []storageBucket{{ID: bucketID, Name: bucketName, URI: "s3://media"}})
		case "POST /api/v1/buckets/bucket-123/objects/upload-url":
			var req storageObjectUploadRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, objectKey, req.Key)
			assert.True(t, strings.HasPrefix(req.ContentType, "text/plain"))
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
		case "PUT /blob":
			assert.Empty(t, r.Header.Get("Authorization"), "API credentials must not reach the object store")
			assert.EqualValues(t, len(contents), r.ContentLength)
			var err error
			stored, err = io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, contents, string(stored))
			w.WriteHeader(http.StatusOK)
		case "POST /api/v1/buckets/bucket-123/objects/confirm":
			var req storageConfirmRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, objectKey, req.Key)
			assert.Empty(t, req.Filename)
			assert.EqualValues(t, len(contents), req.Size)
			assert.NotEmpty(t, stored, "confirm must follow the actual PUT")
			confirmed = true
			storageResponse(t, w, nil)
		case "GET /api/v1/buckets/bucket-123/objects":
			assert.Equal(t, folder, r.URL.Query().Get("prefix"))
			assert.True(t, confirmed)
			storageResponse(t, w, storageObjectListing{Objects: []storageObject{{Key: objectKey, Name: "file.txt", Size: int64(len(stored))}}})
		case "GET /api/v1/buckets/bucket-123/objects/url":
			assert.Equal(t, objectKey, r.URL.Query().Get("key"))
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
		case "GET /blob":
			assert.Empty(t, r.Header.Get("Authorization"))
			w.Header().Set("Content-Length", fmt.Sprint(len(stored)))
			_, err := w.Write(stored)
			assert.NoError(t, err)
		case "DELETE /api/v1/buckets/bucket-123/objects":
			assert.Equal(t, objectKey, r.URL.Query().Get("key"))
			assert.Len(t, r.URL.Query(), 1, "special characters must not create extra parameters")
			deleted = true
			stored = nil
			storageResponse(t, w, nil)
		default:
			t.Errorf("unexpected endpoint (no legacy fallback allowed): %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	})

	localFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(localFile, []byte(contents), 0600))
	out, err := runStorageCommand(t, "buckets")
	require.NoError(t, err)
	assert.Contains(t, out, bucketName)
	assert.Contains(t, out, bucketID)
	out, err = runStorageCommand(t, "upload", localFile, "--bucket", bucketName, "--path", folder)
	require.NoError(t, err)
	assert.Contains(t, out, "Uploaded file.txt")
	out, err = runStorageCommand(t, "ls", folder, "--bucket", bucketID)
	require.NoError(t, err)
	assert.Contains(t, out, "file.txt")
	download := filepath.Join(t.TempDir(), "download.txt")
	out, err = runStorageCommand(t, "download", objectKey, "--bucket", bucketName, "--output", download)
	require.NoError(t, err)
	assert.Contains(t, out, "Downloaded")
	got, err := os.ReadFile(download)
	require.NoError(t, err)
	assert.Equal(t, contents, string(got))
	out, err = runStorageCommand(t, "rm", objectKey, "--bucket", bucketID, "--force")
	require.NoError(t, err)
	assert.Contains(t, out, "Deleted "+objectKey+" from bucket media")
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, deleted)
	assert.Empty(t, stored)
}

func TestStorageBucketUploadFailures(t *testing.T) {
	for _, tc := range []struct {
		name          string
		putStatus     int
		confirmStatus int
		wantConfirm   bool
		wantError     string
	}{
		{"put rejected", http.StatusForbidden, http.StatusOK, false, "upload failed (403)"},
		{"confirm rejected", http.StatusOK, http.StatusBadRequest, true, "confirm upload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			confirmed := false
			setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch r.Method + " " + r.URL.Path {
				case "GET /api/v1/buckets":
					storageResponse(t, w, []storageBucket{{ID: "bucket-1", Name: "media"}})
				case "POST /api/v1/buckets/bucket-1/objects/upload-url":
					var req storageObjectUploadRequest
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
					assert.Equal(t, "file.txt", req.Key)
					storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
				case "PUT /blob":
					w.WriteHeader(tc.putStatus)
				case "POST /api/v1/buckets/bucket-1/objects/confirm":
					confirmed = true
					w.WriteHeader(tc.confirmStatus)
					_, err := io.WriteString(w, `{"success":false,"error":{"message":"confirmation rejected"}}`)
					assert.NoError(t, err)
				default:
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
				}
			})
			file := filepath.Join(t.TempDir(), "file.txt")
			require.NoError(t, os.WriteFile(file, []byte("test"), 0600))
			out, err := runStorageCommand(t, "upload", file, "--bucket", "media")
			require.ErrorContains(t, err, tc.wantError)
			assert.NotContains(t, out, "Uploaded file.txt")
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, tc.wantConfirm, confirmed)
		})
	}
}

func TestStorageBucketSelection(t *testing.T) {
	for _, tc := range []struct{ name, selection, wantError string }{
		{"unknown bucket", "missing", "not found"},
		{"partial ID is not silently matched", "bucket-", "not found"},
		{"empty explicit selection", "", "must not be empty"},
		{"blank explicit selection", "  ", "must not be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := newMockAPI()
			api.on("GET", client.PathBuckets, http.StatusOK, []storageBucket{{ID: "bucket-123", Name: "media"}})
			t.Cleanup(setupTest(t, api))
			_, err := runStorageCommand(t, "ls", "--bucket", tc.selection)
			require.ErrorContains(t, err, tc.wantError)
		})
	}
}

func TestStorageBucketListing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		listing storageObjectListing
		json    bool
		want    []string
	}{
		{"empty", storageObjectListing{}, false, []string{"No files found."}},
		{"folders and files", storageObjectListing{Folders: []storageFolder{{Name: "images"}}, Objects: []storageObject{{Name: "file.txt", Key: "file.txt", Size: 12}}}, false, []string{"images/", "file.txt", "12 B"}},
		{"truncated", storageObjectListing{IsTruncated: true}, false, []string{"listing is truncated"}},
		{"json shape", storageObjectListing{Objects: []storageObject{{Name: "file.txt", Key: "file.txt"}}, IsTruncated: true}, true, []string{`"objects"`, `"key": "file.txt"`, `"isTruncated": true`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := newMockAPI()
			api.on("GET", client.PathBuckets, http.StatusOK, []storageBucket{{ID: "bucket-1", Name: "media"}})
			api.on("GET", client.PathBucketObjects("bucket-1"), http.StatusOK, tc.listing)
			t.Cleanup(setupTest(t, api))
			jsonOutput = tc.json
			out, err := runStorageCommand(t, "ls", "--bucket", "media")
			require.NoError(t, err)
			for _, want := range tc.want {
				assert.Contains(t, out, want)
			}
		})
	}
}

func TestStorageBucketCommandsRequireAuth(t *testing.T) {
	for _, factory := range []func() *cobra.Command{newStorageBucketsCmd, newStorageLsCmd, newStorageUploadCmd, newStorageDownloadCmd, newStorageRmCmd} {
		cmd := factory()
		t.Run(cmd.Name(), func(t *testing.T) {
			t.Cleanup(setupTest(t, newMockAPI()))
			apiClient = nil
			args := []string{}
			if cmd.Name() != "buckets" {
				args = []string{"--bucket", "media"}
				if cmd.Name() != "ls" {
					args = append(args, "file.txt")
				}
			}
			cmd.SetArgs(args)
			require.ErrorIs(t, cmd.Execute(), client.ErrNotAuthenticated)
		})
	}
}

func TestStorageLegacyUploadStillUsesDefaultEndpoints(t *testing.T) {
	setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/storage/presign":
			var req storageUploadRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "file.txt", req.Filename)
			assert.Equal(t, "backups/", req.Path)
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob", "key": "users/org/backups/id_file.txt"})
		case "PUT /blob":
			w.WriteHeader(http.StatusOK)
		case "POST /api/v1/storage/confirm":
			var req storageConfirmRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "users/org/backups/id_file.txt", req.Key)
			assert.Equal(t, "file.txt", req.Filename)
			storageResponse(t, w, nil)
		default:
			t.Errorf("unexpected legacy request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	file := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("legacy"), 0600))
	out, err := runStorageCommand(t, "upload", file, "--path", "backups/")
	require.NoError(t, err)
	assert.Contains(t, out, "Uploaded file.txt")
}

func TestStorageLegacyReadAndDelete(t *testing.T) {
	const contents = "legacy content"
	setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/storage/files":
			storageResponse(t, w, storageListing{Files: []storageFile{{ID: "file-123", Name: "file.txt", Size: int64(len(contents))}}})
		case "GET /api/v1/storage/files/file-123/url":
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
		case "GET /blob":
			_, err := io.WriteString(w, contents)
			assert.NoError(t, err)
		case "DELETE /api/v1/storage/files/file-123":
			storageResponse(t, w, nil)
		default:
			t.Errorf("unexpected legacy request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	out, err := runStorageCommand(t, "ls")
	require.NoError(t, err)
	assert.Contains(t, out, "file.txt")
	dest := filepath.Join(t.TempDir(), "download.txt")
	_, err = runStorageCommand(t, "download", "file.txt", "--output", dest)
	require.NoError(t, err)
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, contents, string(got))
	out, err = runStorageCommand(t, "rm", "file-123", "--force")
	require.NoError(t, err)
	assert.Contains(t, out, "Deleted file.txt")
}

func TestStorageUnknownBucketNeverFallsBack(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("test"), 0600))
	for _, args := range [][]string{
		{"upload", file},
		{"download", "file.txt"},
		{"rm", "file.txt", "--force"},
	} {
		t.Run(args[0], func(t *testing.T) {
			setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == client.PathBuckets {
					storageResponse(t, w, []storageBucket{{ID: "bucket-123", Name: "media"}})
					return
				}
				t.Errorf("unknown bucket must not trigger any file operation: %s %s", r.Method, r.URL)
				http.NotFound(w, r)
			})
			_, err := runStorageCommand(t, append(args, "--bucket", "missing")...)
			require.ErrorContains(t, err, "not found")
		})
	}
}

func TestStorageBucketDownloadFailurePreservesDestination(t *testing.T) {
	setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/buckets":
			storageResponse(t, w, []storageBucket{{ID: "bucket-123", Name: "media"}})
		case "GET /api/v1/buckets/bucket-123/objects/url":
			assert.Equal(t, "images/صور +#.txt", r.URL.Query().Get("key"))
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
		case "GET /blob":
			http.NotFound(w, r)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	dest := filepath.Join(t.TempDir(), "existing.txt")
	require.NoError(t, os.WriteFile(dest, []byte("keep this"), 0600))
	out, err := runStorageCommand(t, "download", "images/صور +#.txt", "--bucket", "media", "--output", dest)
	require.ErrorContains(t, err, "download failed (404)")
	assert.NotContains(t, out, "Downloaded")
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "keep this", string(got))
}

func TestStorageBucketDiscoveryOutput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		buckets []storageBucket
		json    bool
		want    string
	}{
		{"empty", []storageBucket{}, false, "Create a bucket in the dashboard first"},
		{"json", []storageBucket{{ID: "bucket-123", Name: "media", URI: "s3://media"}}, true, `"uri": "s3://media"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := newMockAPI()
			api.on("GET", client.PathBuckets, http.StatusOK, tc.buckets)
			t.Cleanup(setupTest(t, api))
			jsonOutput = tc.json
			out, err := runStorageCommand(t, "buckets")
			require.NoError(t, err)
			assert.Contains(t, out, tc.want)
			if tc.json {
				var got []storageBucket
				require.NoError(t, json.Unmarshal([]byte(out), &got))
				assert.Equal(t, tc.buckets, got)
			}
		})
	}
}

func TestStorageBucketEmptyUpload(t *testing.T) {
	setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/buckets":
			storageResponse(t, w, []storageBucket{{ID: "bucket-123", Name: "media"}})
		case "POST /api/v1/buckets/bucket-123/objects/upload-url":
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
		case "PUT /blob":
			assert.Zero(t, r.ContentLength, "an empty file must have a known zero length, not chunked encoding")
			assert.Empty(t, r.TransferEncoding)
			contents, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Empty(t, contents)
			w.WriteHeader(http.StatusOK)
		case "POST /api/v1/buckets/bucket-123/objects/confirm":
			var req storageConfirmRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Zero(t, req.Size)
			assert.Equal(t, "empty.txt", req.Key)
			storageResponse(t, w, nil)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	file := filepath.Join(t.TempDir(), "empty.txt")
	require.NoError(t, os.WriteFile(file, nil, 0600))
	out, err := runStorageCommand(t, "upload", file, "--bucket", "media")
	require.NoError(t, err)
	assert.Contains(t, out, "Uploaded empty.txt (0 B)")
}

func TestStorageInterruptedDownloadPreservesDestination(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /api/v1/buckets":
					storageResponse(t, w, []storageBucket{{ID: "bucket-123", Name: "media"}})
				case "GET /api/v1/buckets/bucket-123/objects/url":
					storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
				case "GET /blob":
					w.Header().Set("Content-Length", "100")
					_, err := io.WriteString(w, "partial")
					assert.NoError(t, err)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			})
			dir := t.TempDir()
			dest := filepath.Join(dir, "download.txt")
			if existing {
				require.NoError(t, os.WriteFile(dest, []byte("keep this"), 0600))
			}
			out, err := runStorageCommand(t, "download", "file.txt", "--bucket", "media", "--output", dest)
			require.ErrorContains(t, err, "unexpected EOF")
			assert.NotContains(t, out, "Downloaded")
			if existing {
				got, err := os.ReadFile(dest)
				require.NoError(t, err)
				assert.Equal(t, "keep this", string(got))
			} else {
				_, err := os.Stat(dest)
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
			partials, err := filepath.Glob(filepath.Join(dir, ".dcs-download-*"))
			require.NoError(t, err)
			assert.Empty(t, partials)
		})
	}
}

// The legacy listing's name is whatever another org member stored, so without
// --output the download must land in the working directory under its last path
// element, or not at all.
func TestStorageDownloadWithoutOutputStaysInWorkingDirectory(t *testing.T) {
	const contents = "attacker bytes"
	tests := []struct {
		name     string
		remote   string
		absolute bool
		bucket   bool
		want     string
	}{
		{name: "parent traversal", remote: "../escaped.txt", want: "escaped.txt"},
		{name: "traversal after a folder", remote: "a/../../x", want: "x"},
		{name: "absolute path", remote: "outside/x", absolute: true, want: "x"},
		{name: "windows separators", remote: `a\..\..\x`, want: filepath.Base(`a\..\..\x`)},
		{name: "plain name", remote: "report.pdf", want: "report.pdf"},
		{name: "bucket key with folders", remote: "../../images/logo.png", bucket: true, want: "logo.png"},
		{name: "parent directory itself", remote: ".."},
		{name: "current directory", remote: "."},
		{name: "root", remote: "/"},
		{name: "empty", remote: ""},
		{name: "terminal escape", remote: "report\x1b[2J.pdf"},
		{name: "newline", remote: "report\n.pdf"},
		{name: "right-to-left override", remote: "invoice\u202Efdp.exe"},
		{name: "first strong isolate", remote: "report\u2068.pdf"},
		{name: "zero width joiner kept", remote: "\U0001F468\u200D\U0001F469.png", want: "\U0001F468\u200D\U0001F469.png"},
		{name: "right-to-left mark kept", remote: "\u200F\u0645\u0644\u0641.txt", want: "\u200F\u0645\u0644\u0641.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			work := filepath.Join(root, "work", "here")
			require.NoError(t, os.MkdirAll(work, 0o755))
			remote := tt.remote
			if tt.absolute {
				// Aimed inside the test root so a regression cannot write to a real path.
				remote = filepath.Join(root, remote)
				require.NoError(t, os.MkdirAll(filepath.Dir(remote), 0o755))
			}
			setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /api/v1/storage/files":
					storageResponse(t, w, storageListing{Files: []storageFile{{ID: "file-123", Name: remote, Size: int64(len(contents))}}})
				case "GET /api/v1/storage/files/file-123/url", "GET /api/v1/buckets/bucket-123/objects/url":
					storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
				case "GET /api/v1/buckets":
					storageResponse(t, w, []storageBucket{{ID: "bucket-123", Name: "media"}})
				case "GET /blob":
					_, err := io.WriteString(w, contents)
					assert.NoError(t, err)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			})
			t.Chdir(work)

			args := []string{"download", "file-123"}
			if tt.bucket {
				args = []string{"download", remote, "--bucket", "media"}
			}
			out, err := runStorageCommand(t, args...)

			var written []string
			require.NoError(t, filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					rel, relErr := filepath.Rel(work, p)
					written = append(written, rel)
					return relErr
				}
				return err
			}))
			if tt.want == "" {
				require.ErrorContains(t, err, "not a safe local file name")
				assert.Empty(t, written, "a refused download must write nothing")
				assert.NotContains(t, out, "\x1b")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{tt.want}, written, "the only file written is the base name inside the working directory")
			got, err := os.ReadFile(filepath.Join(work, tt.want))
			require.NoError(t, err)
			assert.Equal(t, contents, string(got))
		})
	}
}

func TestStorageRefusedNameDownloadsWithOutput(t *testing.T) {
	const contents = "kept bytes"
	setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/storage/files":
			storageResponse(t, w, storageListing{Files: []storageFile{{ID: "file-123", Name: "..", Size: int64(len(contents))}}})
		case "GET /api/v1/storage/files/file-123/url":
			storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
		case "GET /blob":
			_, err := io.WriteString(w, contents)
			assert.NoError(t, err)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	dest := filepath.Join(t.TempDir(), "chosen.txt")
	_, err := runStorageCommand(t, "download", "file-123")
	require.ErrorContains(t, err, "pass --output")
	_, err = runStorageCommand(t, "download", "file-123", "--output", dest)
	require.NoError(t, err)
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, contents, string(got))
}

// Without --output the destination is a name another org member chose, so an
// existing entry under that name is never replaced unless --force says so.
func TestStorageDownloadDoesNotReplaceWithoutForce(t *testing.T) {
	const contents = "remote bytes"
	var blobRequests int
	serve := func(t *testing.T) {
		blobRequests = 0
		setupStorageHTTP(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.Method + " " + r.URL.Path {
			case "GET /api/v1/storage/files":
				storageResponse(t, w, storageListing{Files: []storageFile{{ID: "file-123", Name: ".zshrc", Size: int64(len(contents))}}})
			case "GET /api/v1/storage/files/file-123/url":
				storageResponse(t, w, map[string]string{"url": "http://" + r.Host + "/blob"})
			case "GET /blob":
				blobRequests++
				_, err := io.WriteString(w, contents)
				assert.NoError(t, err)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				http.NotFound(w, r)
			}
		})
	}
	existing := map[string]func(t *testing.T, dir string){
		"file": func(t *testing.T, dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".zshrc"), []byte("mine"), 0o600))
		},
		"symlink": func(t *testing.T, dir string) {
			target := filepath.Join(dir, "target")
			require.NoError(t, os.WriteFile(target, []byte("mine"), 0o600))
			if err := os.Symlink(target, filepath.Join(dir, ".zshrc")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
		"dangling symlink": func(t *testing.T, dir string) {
			if err := os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(dir, ".zshrc")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
		"directory": func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(filepath.Join(dir, ".zshrc"), 0o755))
		},
	}
	snapshot := func(t *testing.T, dir string) map[string]string {
		t.Helper()
		got := map[string]string{}
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			info, err := os.Lstat(p)
			require.NoError(t, err)
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				link, err := os.Readlink(p)
				require.NoError(t, err)
				got[e.Name()] = "symlink -> " + link
			case info.IsDir():
				got[e.Name()] = "dir"
			default:
				data, err := os.ReadFile(p)
				require.NoError(t, err)
				got[e.Name()] = string(data)
			}
		}
		return got
	}
	for kind, plant := range existing {
		t.Run(kind+" is left untouched", func(t *testing.T) {
			serve(t)
			dir := t.TempDir()
			t.Chdir(dir)
			plant(t, dir)
			before := snapshot(t, dir)

			out, err := runStorageCommand(t, "download", "file-123")

			require.ErrorContains(t, err, `".zshrc" already exists`)
			if kind == "directory" {
				require.ErrorContains(t, err, "--output")
				assert.NotContains(t, err.Error(), "--force", "--force cannot replace a folder")
			} else {
				require.ErrorContains(t, err, "--force")
			}
			assert.NotContains(t, out, "Downloaded")
			assert.Zero(t, blobRequests, "an early refusal must not download anything")
			assert.Equal(t, before, snapshot(t, dir), "nothing in the directory may change, and no temporary file may remain")
		})
	}
	t.Run("force replaces a file", func(t *testing.T) {
		serve(t)
		dir := t.TempDir()
		t.Chdir(dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".zshrc"), []byte("mine"), 0o600))

		_, err := runStorageCommand(t, "download", "file-123", "--force")

		require.NoError(t, err)
		assert.Equal(t, map[string]string{".zshrc": contents}, snapshot(t, dir))
	})
	t.Run("force replaces a symlink, not its target", func(t *testing.T) {
		serve(t)
		dir := t.TempDir()
		t.Chdir(dir)
		target := filepath.Join(dir, "target")
		require.NoError(t, os.WriteFile(target, []byte("mine"), 0o600))
		if err := os.Symlink(target, filepath.Join(dir, ".zshrc")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := runStorageCommand(t, "download", "file-123", "-f")

		require.NoError(t, err)
		assert.Equal(t, map[string]string{".zshrc": contents, "target": "mine"}, snapshot(t, dir))
	})
	t.Run("no existing entry", func(t *testing.T) {
		serve(t)
		dir := t.TempDir()
		t.Chdir(dir)

		_, err := runStorageCommand(t, "download", "file-123")

		require.NoError(t, err)
		assert.Equal(t, map[string]string{".zshrc": contents}, snapshot(t, dir))
	})
	t.Run("output still replaces what the user named", func(t *testing.T) {
		serve(t)
		dir := t.TempDir()
		dest := filepath.Join(dir, "chosen.txt")
		require.NoError(t, os.WriteFile(dest, []byte("mine"), 0o600))

		_, err := runStorageCommand(t, "download", "file-123", "--output", dest)

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"chosen.txt": contents}, snapshot(t, dir))
	})
}

// The early check in the command only spares a transfer. This is the step that
// must hold when the name appears after that check.
func TestSaveStorageDownloadNeverReplacesWithoutForce(t *testing.T) {
	plants := map[string]func(t *testing.T, dest string){
		"file": func(t *testing.T, dest string) {
			require.NoError(t, os.WriteFile(dest, []byte("mine"), 0o600))
		},
		"symlink to a file": func(t *testing.T, dest string) {
			target := dest + ".target"
			require.NoError(t, os.WriteFile(target, []byte("mine"), 0o600))
			if err := os.Symlink(target, dest); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
		"dangling symlink": func(t *testing.T, dest string) {
			if err := os.Symlink(dest+".missing", dest); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
		"directory": func(t *testing.T, dest string) {
			require.NoError(t, os.Mkdir(dest, 0o755))
		},
	}
	for kind, plant := range plants {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "name")
			plant(t, dest)
			before, err := os.Lstat(dest)
			require.NoError(t, err)

			_, err = saveStorageDownload(dest, strings.NewReader("remote"), false, os.Link)

			require.ErrorContains(t, err, "already exists")
			after, err := os.Lstat(dest)
			require.NoError(t, err)
			assert.Equal(t, before.Mode(), after.Mode())
			assert.Equal(t, before.ModTime(), after.ModTime())
			if target, err := os.ReadFile(dest + ".target"); err == nil {
				assert.Equal(t, "mine", string(target), "a symlink's target must not be written through")
			}
			_, err = os.Lstat(dest + ".missing")
			assert.ErrorIs(t, err, os.ErrNotExist, "a dangling symlink must not be followed")
			partials, err := filepath.Glob(filepath.Join(dir, ".dcs-download-*"))
			require.NoError(t, err)
			assert.Empty(t, partials)
		})
	}
	t.Run("free name", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "name")
		written, err := saveStorageDownload(dest, strings.NewReader("remote"), false, os.Link)
		require.NoError(t, err)
		assert.EqualValues(t, len("remote"), written)
		got, err := os.ReadFile(dest)
		require.NoError(t, err)
		assert.Equal(t, "remote", string(got))
	})
}

func TestSaveStorageDownloadLinkSeam(t *testing.T) {
	t.Run("name appears just before the link", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "name")
		calls := 0
		plantThenLink := func(oldname, newname string) error {
			calls++
			require.NoError(t, os.WriteFile(newname, []byte("planted"), 0o600))
			return os.Link(oldname, newname)
		}

		_, err := saveStorageDownload(dest, strings.NewReader("remote"), false, plantThenLink)

		require.ErrorContains(t, err, fmt.Sprintf("%q already exists", dest))
		assert.Equal(t, 1, calls)
		got, err := os.ReadFile(dest)
		require.NoError(t, err)
		assert.Equal(t, "planted", string(got))
		partials, err := filepath.Glob(filepath.Join(dir, ".dcs-download-*"))
		require.NoError(t, err)
		assert.Empty(t, partials)
	})
	t.Run("filesystem without hard links", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "name")
		unsupported := func(oldname, newname string) error {
			return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: errors.ErrUnsupported}
		}

		_, err := saveStorageDownload(dest, strings.NewReader("remote"), false, unsupported)

		require.ErrorContains(t, err, "without replacing anything")
		require.ErrorContains(t, err, "--force")
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries, "neither the destination nor a temporary file may remain")
	})
}

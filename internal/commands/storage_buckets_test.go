package commands

import (
	"encoding/json"
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

package client

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathConstants(t *testing.T) {
	assert.Equal(t, "/api/v1", PathAPIPrefix)
	assert.Equal(t, "/api/v1/auth/me", PathAuthMe)
	assert.Equal(t, "/api/v1/servers", PathServers)
	assert.Equal(t, "/api/v1/ssh-keys", PathSSHKeys)
	assert.Equal(t, "/api/v1/ws", PathWS)
}

func TestPathServer(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123", PathServer("srv-123"))
}

func TestPathServerReboot(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/reboot", PathServerReboot("srv-123"))
}

func TestPathServerCredentials(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/credentials", PathServerCredentials("srv-123"))
}

func TestPathServerEvents(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/events", PathServerEvents("srv-123"))
}

func TestPathSites(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/sites", PathSites("srv-123"))
}

func TestPathSite(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/sites/site-456", PathSite("srv-123", "site-456"))
}

func TestPathSiteDeploy(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/sites/site-456/deploy", PathSiteDeploy("srv-123", "site-456"))
}

func TestPathSiteDeployments(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/sites/site-456/deployments", PathSiteDeployments("srv-123", "site-456"))
}

func TestPathSiteEnv(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/sites/site-456/env", PathSiteEnv("srv-123", "site-456"))
}

func TestPathDatabases(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/databases", PathDatabases("srv-123"))
}

func TestPathDatabase(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/databases/db-789", PathDatabase("srv-123", "db-789"))
}

func TestPathDatabaseUsers(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/databases/db-789/users", PathDatabaseUsers("srv-123", "db-789"))
}

func TestPathDatabaseSchema(t *testing.T) {
	assert.Equal(t, "/api/v1/servers/srv-123/databases/db-789/schema", PathDatabaseSchema("srv-123", "db-789"))
}

func TestPathSSHKey(t *testing.T) {
	assert.Equal(t, "/api/v1/ssh-keys/key-abc", PathSSHKey("key-abc"))
}

func TestPathStorageFile(t *testing.T) {
	assert.Equal(t, "/api/v1/storage/files/file-123", PathStorageFile("file-123"))
}

func TestPathStorageFileDownload(t *testing.T) {
	assert.Equal(t, "/api/v1/storage/files/file-123/download", PathStorageFileDownload("file-123"))
}

func TestPathStorageFileURL(t *testing.T) {
	assert.Equal(t, "/api/v1/storage/files/file-123/url", PathStorageFileURL("file-123"))
}

func TestBucketPaths(t *testing.T) {
	assert.Equal(t, "/api/v1/buckets", PathBuckets)
	assert.Equal(t, "/api/v1/buckets/bucket-1/objects", PathBucketObjects("bucket-1"))
	assert.Equal(t, "/api/v1/buckets/bucket-1/objects/url", PathBucketObjectURL("bucket-1"))
	assert.Equal(t, "/api/v1/buckets/bucket-1/objects/upload-url", PathBucketUploadURL("bucket-1"))
	assert.Equal(t, "/api/v1/buckets/bucket-1/objects/confirm", PathBucketConfirm("bucket-1"))
	for _, key := range []string{"file.txt", "images/a b+#?.txt", "صور/file.txt", "nested/a%2Fb.txt"} {
		u, err := url.Parse(PathBucketObject("bucket-1", key))
		assert.NoError(t, err)
		assert.Equal(t, key, u.Query().Get("key"))
		assert.Len(t, u.Query(), 1)
	}
}

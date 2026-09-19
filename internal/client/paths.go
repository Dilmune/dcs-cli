package client

import (
	"fmt"
	"net/url"
)

const (
	PathAPIPrefix          = "/api/v1"
	PathAuthMe             = PathAPIPrefix + "/auth/me"
	PathServers            = PathAPIPrefix + "/servers"
	PathSSHKeys            = PathAPIPrefix + "/ssh-keys"
	PathAPIKeys            = PathAPIPrefix + "/api-keys"
	PathProvisionerRegions = PathAPIPrefix + "/provisioner/regions"
	PathCatalog            = PathAPIPrefix + "/catalog"
	PathStorageFiles       = PathAPIPrefix + "/storage/files"
	PathStoragePresign     = PathAPIPrefix + "/storage/presign"
	PathStorageConfirm     = PathAPIPrefix + "/storage/confirm"
	PathStorageUsage       = PathAPIPrefix + "/storage/usage"
	PathBuckets            = PathAPIPrefix + "/buckets"
	PathDashboardStats     = PathAPIPrefix + "/dashboard/stats"
	PathWS                 = PathAPIPrefix + "/ws"
	PortalBaseURL          = "https://cloud.dilmune.com"
	SSHUser                = "dilmune"
)

// Servers

func PathServer(serverID string) string {
	return fmt.Sprintf("%s/%s", PathServers, serverID)
}

func PathServerReboot(serverID string) string {
	return fmt.Sprintf("%s/%s/reboot", PathServers, serverID)
}

func PathServerCredentials(serverID string) string {
	return fmt.Sprintf("%s/%s/credentials", PathServers, serverID)
}

func PathServerInstallSoftware(serverID string) string {
	return fmt.Sprintf("%s/%s/install-software", PathServers, serverID)
}

func PathServerUninstallSoftware(serverID string) string {
	return fmt.Sprintf("%s/%s/uninstall-software", PathServers, serverID)
}

func PathServerEvents(serverID string) string {
	return fmt.Sprintf("%s/%s/events", PathServers, serverID)
}

// Sites

func PathSites(serverID string) string {
	return fmt.Sprintf("%s/%s/sites", PathServers, serverID)
}

func PathSite(serverID, siteID string) string {
	return fmt.Sprintf("%s/%s/sites/%s", PathServers, serverID, siteID)
}

func PathSiteDeploy(serverID, siteID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/deploy", PathServers, serverID, siteID)
}

func PathSiteDeployments(serverID, siteID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/deployments", PathServers, serverID, siteID)
}

func PathSiteEnv(serverID, siteID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/env", PathServers, serverID, siteID)
}

func PathSiteSSL(serverID, siteID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/ssl", PathServers, serverID, siteID)
}

func PathDeployment(serverID, siteID, deploymentID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/deployments/%s", PathServers, serverID, siteID, deploymentID)
}

func PathDeploymentLogs(serverID, siteID, deploymentID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/deployments/%s/logs", PathServers, serverID, siteID, deploymentID)
}

func PathDeploymentRollback(serverID, siteID, deploymentID string) string {
	return fmt.Sprintf("%s/%s/sites/%s/deployments/%s/rollback", PathServers, serverID, siteID, deploymentID)
}

// Databases

func PathDatabases(serverID string) string {
	return fmt.Sprintf("%s/%s/databases", PathServers, serverID)
}

func PathDatabase(serverID, dbID string) string {
	return fmt.Sprintf("%s/%s/databases/%s", PathServers, serverID, dbID)
}

func PathDatabaseUsers(serverID, dbID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/users", PathServers, serverID, dbID)
}

func PathDatabaseSchema(serverID, dbID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/schema", PathServers, serverID, dbID)
}

func PathDatabaseBackups(serverID, dbID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/backups", PathServers, serverID, dbID)
}

func PathDatabaseBackup(serverID, dbID, backupID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/backups/%s", PathServers, serverID, dbID, backupID)
}

func PathDatabaseBackupDownload(serverID, dbID, backupID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/backups/%s/download", PathServers, serverID, dbID, backupID)
}

func PathDatabaseBackupRestore(serverID, dbID, backupID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/backups/%s/restore", PathServers, serverID, dbID, backupID)
}

func PathDatabaseQuery(serverID, dbID string) string {
	return fmt.Sprintf("%s/%s/databases/%s/query", PathServers, serverID, dbID)
}

// SSH Keys

func PathSSHKey(keyID string) string {
	return fmt.Sprintf("%s/%s", PathSSHKeys, keyID)
}

// API Keys

func PathAPIKey(keyID string) string {
	return fmt.Sprintf("%s/%s", PathAPIKeys, keyID)
}

// Storage

func PathStorageFile(fileID string) string {
	return fmt.Sprintf("%s/%s", PathStorageFiles, fileID)
}

func PathStorageFileDownload(fileID string) string {
	return fmt.Sprintf("%s/%s/download", PathStorageFiles, fileID)
}

func PathStorageFileURL(fileID string) string {
	return fmt.Sprintf("%s/%s/url", PathStorageFiles, fileID)
}

func PathBucketObjects(bucketID string) string {
	return fmt.Sprintf("%s/%s/objects", PathBuckets, bucketID)
}

func PathBucketObjectURL(bucketID string) string {
	return PathBucketObjects(bucketID) + "/url"
}

func PathBucketUploadURL(bucketID string) string {
	return PathBucketObjects(bucketID) + "/upload-url"
}

func PathBucketConfirm(bucketID string) string {
	return PathBucketObjects(bucketID) + "/confirm"
}

func PathBucketObject(bucketID, key string) string {
	return PathBucketObjects(bucketID) + "?key=" + url.QueryEscape(key)
}

// Firewall

func PathFirewall(serverID string) string {
	return fmt.Sprintf("%s/%s/firewall", PathServers, serverID)
}

func PathFirewallRules(serverID string) string {
	return fmt.Sprintf("%s/%s/firewall/rules", PathServers, serverID)
}

func PathFirewallRule(serverID string, ruleNumber int) string {
	return fmt.Sprintf("%s/%s/firewall/rules/%d", PathServers, serverID, ruleNumber)
}

func PathFirewallToggle(serverID string) string {
	return fmt.Sprintf("%s/%s/firewall/toggle", PathServers, serverID)
}

// Cron Jobs

func PathCronJobs(serverID string) string {
	return fmt.Sprintf("%s/%s/cron-jobs", PathServers, serverID)
}

func PathCronJob(serverID, cronID string) string {
	return fmt.Sprintf("%s/%s/cron-jobs/%s", PathServers, serverID, cronID)
}

func PathCronJobRun(serverID, cronID string) string {
	return fmt.Sprintf("%s/%s/cron-jobs/%s/run", PathServers, serverID, cronID)
}

func PathCronJobExecutions(serverID, cronID string) string {
	return fmt.Sprintf("%s/%s/cron-jobs/%s/executions", PathServers, serverID, cronID)
}

// Daemons

func PathDaemons(serverID string) string {
	return fmt.Sprintf("%s/%s/daemons", PathServers, serverID)
}

func PathDaemon(serverID, daemonID string) string {
	return fmt.Sprintf("%s/%s/daemons/%s", PathServers, serverID, daemonID)
}

func PathDaemonLogs(serverID, daemonID string) string {
	return fmt.Sprintf("%s/%s/daemons/%s/logs", PathServers, serverID, daemonID)
}

func PathDaemonAction(serverID, daemonID, action string) string {
	return fmt.Sprintf("%s/%s/daemons/%s/%s", PathServers, serverID, daemonID, action)
}

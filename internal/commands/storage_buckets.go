package commands

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

const (
	storageBucketFlag  = "bucket"
	storageBucketHint  = "Run 'dcs storage buckets' to see bucket names and IDs."
	storagePrefixQuery = "prefix"
	storageKeyQuery    = "key"
)

type storageBucket struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URI  string `json:"uri"`
}

func (b storageBucket) Matches(nameOrID string) bool {
	return b.Name == nameOrID || b.ID == nameOrID
}

type storageObject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type storageObjectListing struct {
	Objects     []storageObject `json:"objects"`
	Folders     []storageFolder `json:"folders"`
	IsTruncated bool            `json:"isTruncated"`
}

type storageUploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Path        string `json:"path,omitempty"`
}

type storageConfirmRequest struct {
	Key         string `json:"key"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

type storageObjectUploadRequest struct {
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
}

type storageUploadTarget struct {
	URL      string `json:"url"`
	Key      string `json:"key"`
	BucketID string `json:"-"`
}

type storageDownloadTarget struct {
	URL  string `json:"url"`
	Name string `json:"-"`
	Size int64  `json:"-"`
}

func addStorageBucketFlag(cmd *cobra.Command, value *string) {
	cmd.Flags().StringVar(value, storageBucketFlag, "", "Bucket name or full ID (omit for legacy default storage)")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(storageBucketFlag) && strings.TrimSpace(*value) == "" {
			return fmt.Errorf("bucket selection: name or ID must not be empty")
		}
		return nil
	}
}

func newStorageBucketsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "buckets",
		Short:   "List available buckets",
		Example: "  dcs storage buckets\n  dcs storage buckets --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
			resp, err := apiClient.Get(cmd.Context(), client.PathBuckets, nil)
			if err != nil {
				return fmt.Errorf("list buckets: %w", err)
			}
			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}
			buckets, err := client.Decode[[]storageBucket](resp)
			if err != nil {
				return fmt.Errorf("decode buckets: %w", err)
			}
			if len(buckets) == 0 {
				ui.PrintInfo("No buckets found. Create a bucket in the dashboard first.")
				return nil
			}
			rows := make([][]string, 0, len(buckets))
			for _, bucket := range buckets {
				rows = append(rows, []string{bucket.Name, bucket.ID, bucket.URI})
			}
			ui.PrintTable([]string{"Name", "ID", "URI"}, rows)
			return nil
		},
	}
}

func resolveStorageBucket(ctx context.Context, api *client.Client, nameOrID string) (*storageBucket, error) {
	resp, err := api.Get(ctx, client.PathBuckets, nil)
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}
	buckets, err := client.Decode[[]storageBucket](resp)
	if err != nil {
		return nil, fmt.Errorf("decode buckets: %w", err)
	}
	for _, bucket := range buckets {
		if bucket.Matches(nameOrID) {
			return &bucket, nil
		}
	}
	return nil, &client.CLIError{
		Message:    fmt.Sprintf("Bucket %q not found.", nameOrID),
		Suggestion: storageBucketHint,
	}
}

func listStorageBucket(ctx context.Context, api *client.Client, bucketName, prefix string) error {
	bucket, err := resolveStorageBucket(ctx, api, bucketName)
	if err != nil {
		return fmt.Errorf("resolve bucket: %w", err)
	}
	resp, err := api.Get(ctx, client.PathBucketObjects(bucket.ID), url.Values{storagePrefixQuery: {prefix}})
	if err != nil {
		return fmt.Errorf("list bucket objects: %w", err)
	}
	if jsonOutput {
		ui.PrintJSONRaw(resp.Data)
		return nil
	}
	listing, err := client.Decode[storageObjectListing](resp)
	if err != nil {
		return fmt.Errorf("decode bucket objects: %w", err)
	}
	rows := make([][]string, 0, len(listing.Folders)+len(listing.Objects))
	for _, folder := range listing.Folders {
		rows = append(rows, []string{folder.Name + "/", "-"})
	}
	for _, object := range listing.Objects {
		rows = append(rows, []string{object.Name, formatSize(object.Size)})
	}
	if len(rows) == 0 {
		ui.PrintInfo("No files found.")
	} else {
		ui.PrintTable([]string{"Name", "Size"}, rows)
	}
	if listing.IsTruncated {
		ui.PrintInfo("Object listing is truncated. Use a narrower folder prefix to see more files.")
	}
	return nil
}

func prepareStorageUpload(ctx context.Context, api *client.Client, bucketName string, req storageUploadRequest) (*storageUploadTarget, error) {
	if bucketName == "" {
		resp, err := api.Post(ctx, client.PathStoragePresign, req)
		if err != nil {
			return nil, fmt.Errorf("presign legacy upload: %w", err)
		}
		target, err := client.Decode[storageUploadTarget](resp)
		if err != nil {
			return nil, fmt.Errorf("decode upload URL: %w", err)
		}
		return &target, nil
	}
	bucket, err := resolveStorageBucket(ctx, api, bucketName)
	if err != nil {
		return nil, fmt.Errorf("resolve bucket: %w", err)
	}
	key := req.Filename
	if prefix := strings.Trim(req.Path, "/"); prefix != "" {
		key = prefix + "/" + req.Filename
	}
	resp, err := api.Post(ctx, client.PathBucketUploadURL(bucket.ID), storageObjectUploadRequest{Key: key, ContentType: req.ContentType})
	if err != nil {
		return nil, fmt.Errorf("presign bucket upload: %w", err)
	}
	target, err := client.Decode[storageUploadTarget](resp)
	if err != nil {
		return nil, fmt.Errorf("decode upload URL: %w", err)
	}
	target.Key = key
	target.BucketID = bucket.ID
	return &target, nil
}

func confirmStorageUpload(ctx context.Context, api *client.Client, target *storageUploadTarget, req storageConfirmRequest) error {
	endpoint := client.PathStorageConfirm
	if target.BucketID != "" {
		endpoint = client.PathBucketConfirm(target.BucketID)
		req.Filename = ""
	}
	if _, err := api.Post(ctx, endpoint, req); err != nil {
		return fmt.Errorf("confirm stored object: %w", err)
	}
	return nil
}

func prepareStorageDownload(ctx context.Context, api *client.Client, bucketName, nameOrKey string) (*storageDownloadTarget, error) {
	var endpoint, name string
	var size int64
	var query url.Values
	if bucketName == "" {
		file, err := resolveStorageFile(nameOrKey)
		if err != nil {
			return nil, fmt.Errorf("resolve storage file: %w", err)
		}
		endpoint, name, size = client.PathStorageFileURL(file.ID), file.Name, file.Size
	} else {
		bucket, err := resolveStorageBucket(ctx, api, bucketName)
		if err != nil {
			return nil, fmt.Errorf("resolve bucket: %w", err)
		}
		endpoint, name = client.PathBucketObjectURL(bucket.ID), filepath.Base(nameOrKey)
		query = url.Values{storageKeyQuery: {nameOrKey}}
	}
	resp, err := api.Get(ctx, endpoint, query)
	if err != nil {
		return nil, fmt.Errorf("get download URL: %w", err)
	}
	target, err := client.Decode[storageDownloadTarget](resp)
	if err != nil {
		return nil, fmt.Errorf("decode download URL: %w", err)
	}
	target.Name, target.Size = name, size
	return &target, nil
}

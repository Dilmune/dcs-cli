package commands

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Manage object storage",
	}

	cmd.AddCommand(newStorageLsCmd())
	cmd.AddCommand(newStorageUploadCmd())
	cmd.AddCommand(newStorageDownloadCmd())
	cmd.AddCommand(newStorageRmCmd())
	cmd.AddCommand(newStorageUsageCmd())
	cmd.AddCommand(newStorageBucketsCmd())

	return cmd
}

type storageFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

type storageListing struct {
	Files   []storageFile   `json:"files"`
	Folders []storageFolder `json:"folders"`
}

type storageFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func newStorageLsCmd() *cobra.Command {
	var bucketName string
	cmd := &cobra.Command{
		Use:     "ls [path]",
		Aliases: []string{"list"},
		Short:   "List files and folders",
		Example: "  dcs storage ls --bucket media\n  dcs storage ls uploads/ --bucket media\n  dcs storage ls",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
			if bucketName != "" {
				prefix := ""
				if len(args) > 0 {
					prefix = args[0]
				}
				if err := listStorageBucket(cmd.Context(), apiClient, bucketName, prefix); err != nil {
					return fmt.Errorf("list storage: %w", err)
				}
				return nil
			}

			var query url.Values
			if len(args) > 0 {
				query = url.Values{"path": {args[0]}}
			}

			resp, err := apiClient.Get(context.Background(), client.PathStorageFiles, query)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			listing, err := client.Decode[storageListing](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(listing.Folders) == 0 && len(listing.Files) == 0 {
				ui.PrintInfo("No files found.")
				return nil
			}

			headers := []string{"Name", "Size", "Type"}
			var rows [][]string

			for _, f := range listing.Folders {
				rows = append(rows, []string{f.Name + "/", "-", "folder"})
			}
			for _, f := range listing.Files {
				rows = append(rows, []string{f.Name, formatSize(f.Size), f.MimeType})
			}

			fmt.Println()
			ui.PrintTable(headers, rows)
			return nil
		},
	}
	addStorageBucketFlag(cmd, &bucketName)
	return cmd
}

func newStorageUploadCmd() *cobra.Command {
	var remotePath, bucketName string
	cmd := &cobra.Command{
		Use:     "upload <local-path>",
		Short:   "Upload a file",
		Long:    "Upload a file. With --bucket, the object key is the filename prefixed by --path. Uploading to an existing key replaces that object.",
		Example: "  dcs storage upload ./file.txt --bucket media\n  dcs storage upload ./backup.sql --bucket media -p backups/",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			localPath := args[0]
			info, err := os.Stat(localPath)
			if err != nil {
				return fmt.Errorf("open %s: %w", localPath, err)
			}
			if info.IsDir() {
				return fmt.Errorf("%s is a directory", localPath)
			}

			filename := filepath.Base(localPath)
			contentType := detectContentType(localPath)

			presign, err := prepareStorageUpload(cmd.Context(), apiClient, bucketName, storageUploadRequest{
				Filename:    filename,
				ContentType: contentType,
				Size:        info.Size(),
				Path:        remotePath,
			})
			if err != nil {
				return fmt.Errorf("request presigned URL: %w", err)
			}

			// Step 2: Upload file directly to presigned URL
			f, err := os.Open(localPath)
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			defer f.Close()

			fmt.Println()
			pw := ui.NewProgressWriter(info.Size(), "Uploading")
			body := pw.WrapReader(f)

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPut, presign.URL, body)
			if err != nil {
				return fmt.Errorf("create upload request: %w", err)
			}
			req.Header.Set("Content-Type", contentType)
			req.ContentLength = info.Size()
			if info.Size() == 0 {
				req.Body = http.NoBody
			}

			uploadResp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println()
				return fmt.Errorf("upload file: %w", err)
			}
			defer uploadResp.Body.Close()

			if uploadResp.StatusCode >= 400 {
				respBody, _ := io.ReadAll(uploadResp.Body)
				fmt.Println()
				return fmt.Errorf("upload failed (%d): %s", uploadResp.StatusCode, string(respBody))
			}

			pw.Finish()

			err = confirmStorageUpload(cmd.Context(), apiClient, presign, storageConfirmRequest{
				Key:         presign.Key,
				Filename:    filename,
				Size:        info.Size(),
				ContentType: contentType,
			})
			if err != nil {
				return fmt.Errorf("confirm upload: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Uploaded %s (%s)", filename, formatSize(info.Size())))
			return nil
		},
	}
	cmd.Flags().StringVarP(&remotePath, "path", "p", "", "Remote folder path")
	addStorageBucketFlag(cmd, &bucketName)
	return cmd
}

func newStorageDownloadCmd() *cobra.Command {
	var outputPath, bucketName string
	cmd := &cobra.Command{
		Use:     "download <filename-or-key>",
		Short:   "Download a file",
		Example: "  dcs storage download images/logo.png --bucket media\n  dcs storage download backup.sql -o ./local-backup.sql",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			file, err := prepareStorageDownload(cmd.Context(), apiClient, bucketName, args[0])
			if err != nil {
				return fmt.Errorf("prepare download: %w", err)
			}

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, file.URL, nil)
			if err != nil {
				return fmt.Errorf("create download request: %w", err)
			}
			dlResp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("download file: %w", err)
			}
			defer dlResp.Body.Close()

			if dlResp.StatusCode >= 400 {
				return fmt.Errorf("download failed (%d)", dlResp.StatusCode)
			}

			dest := outputPath
			if dest == "" {
				dest = file.Name
			}

			fmt.Println()
			if bucketName != "" {
				file.Size = dlResp.ContentLength
			}
			pw := ui.NewProgressWriter(file.Size, "Downloading")
			body := pw.WrapReader(dlResp.Body)

			written, err := saveStorageDownload(dest, body)
			if err != nil {
				fmt.Println()
				return fmt.Errorf("write file: %w", err)
			}

			pw.Finish()
			ui.PrintSuccess(fmt.Sprintf("Downloaded %s (%s)", dest, formatSize(written)))
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path")
	addStorageBucketFlag(cmd, &bucketName)
	return cmd
}

func saveStorageDownload(dest string, body io.Reader) (int64, error) {
	// Keep an existing destination intact until the complete transfer is closed.
	out, err := os.CreateTemp(filepath.Dir(dest), ".dcs-download-*")
	if err != nil {
		return 0, fmt.Errorf("create temporary download: %w", err)
	}
	defer os.Remove(out.Name())
	defer out.Close()

	written, err := io.Copy(out, body)
	if err != nil {
		return 0, fmt.Errorf("write temporary download: %w", err)
	}
	if err := out.Close(); err != nil {
		return 0, fmt.Errorf("close temporary download: %w", err)
	}
	if err := os.Rename(out.Name(), dest); err != nil {
		return 0, fmt.Errorf("save download to %s: %w", dest, err)
	}
	return written, nil
}

func newStorageRmCmd() *cobra.Command {
	var force bool
	var bucketName string
	cmd := &cobra.Command{
		Use:     "rm <filename-or-key>",
		Short:   "Delete a file",
		Example: "  dcs storage rm images/logo.png --bucket media\n  dcs storage rm logo.png --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			var endpoint, label string
			if bucketName == "" {
				file, err := resolveStorageFile(args[0])
				if err != nil {
					return fmt.Errorf("resolve storage file: %w", err)
				}
				endpoint, label = client.PathStorageFile(file.ID), file.Name
			} else {
				bucket, err := resolveStorageBucket(cmd.Context(), apiClient, bucketName)
				if err != nil {
					return fmt.Errorf("resolve bucket: %w", err)
				}
				endpoint = client.PathBucketObject(bucket.ID, args[0])
				label = fmt.Sprintf("%s from bucket %s", args[0], bucket.Name)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete %s?", label))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			if _, err := apiClient.Delete(cmd.Context(), endpoint); err != nil {
				return fmt.Errorf("delete file: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Deleted %s", label))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	addStorageBucketFlag(cmd, &bucketName)
	return cmd
}

func newStorageUsageCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "usage",
		Short:   "Show storage usage",
		Example: "  dcs storage usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathStorageUsage, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			usage, err := client.Decode[struct {
				UsedBytes  int64  `json:"used_bytes"`
				LimitBytes int64  `json:"limit_bytes"`
				Tier       string `json:"tier"`
			}](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			ui.PrintSection("Storage Usage")
			ui.PrintKeyValue("Used", formatSize(usage.UsedBytes))
			ui.PrintKeyValue("Limit", formatSize(usage.LimitBytes))
			ui.PrintKeyValue("Tier", usage.Tier)
			if usage.LimitBytes > 0 {
				pct := float64(usage.UsedBytes) / float64(usage.LimitBytes) * 100
				ui.PrintKeyValue("Usage", fmt.Sprintf("%.1f%%", pct))
			}
			fmt.Println()
			return nil
		},
	}
}

// resolveStorageFile finds a file by name or ID from the user's storage.
func resolveStorageFile(nameOrID string) (*storageFile, error) {
	resp, err := apiClient.Get(context.Background(), client.PathStorageFiles, nil)
	if err != nil {
		return nil, err
	}

	listing, err := client.Decode[storageListing](resp)
	if err != nil {
		return nil, err
	}

	for _, f := range listing.Files {
		if strings.EqualFold(f.Name, nameOrID) || f.ID == nameOrID || strings.HasPrefix(f.ID, nameOrID) {
			return &f, nil
		}
	}

	return nil, &client.CLIError{
		Message:    fmt.Sprintf("File %q not found.", nameOrID),
		Suggestion: "Run 'dcs storage ls' to see your files.",
	}
}

func detectContentType(path string) string {
	ext := filepath.Ext(path)
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

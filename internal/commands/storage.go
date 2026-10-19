package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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
	var force bool
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
			dest := outputPath
			replace := outputPath != "" || force
			if dest == "" {
				if dest, err = localDownloadName(file.Name); err != nil {
					return fmt.Errorf("choose local file: %w", err)
				}
			}
			if !replace {
				// Only spares the transfer; saveStorageDownload is what refuses.
				if _, err := os.Lstat(dest); err == nil {
					return existingDownloadError(dest)
				}
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

			fmt.Println()
			if bucketName != "" {
				file.Size = dlResp.ContentLength
			}
			pw := ui.NewProgressWriter(file.Size, "Downloading")
			body := pw.WrapReader(dlResp.Body)

			written, err := saveStorageDownload(dest, body, replace, os.Link)
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
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Replace an existing file of the same name in the current directory")
	addStorageBucketFlag(cmd, &bucketName)
	return cmd
}

// localDownloadName turns a remote name into a file in the working directory.
// The name is server data another org member chose, so it never gets to pick a
// directory, and a name that would rewrite the terminal or read differently
// from its bytes is refused.
func localDownloadName(remote string) (string, error) {
	name := filepath.Base(remote)
	if name == "." || !filepath.IsLocal(name) || strings.ContainsFunc(name, isUnsafeNameRune) {
		return "", fmt.Errorf("remote name %q is not a safe local file name, pass --output", remote)
	}
	return name, nil
}

// isUnsafeNameRune leaves other format characters alone: ZWJ, ZWNJ, LRM and
// RLM are ordinary in Arabic, Persian and emoji names.
func isUnsafeNameRune(r rune) bool {
	return unicode.IsControl(r) || (r >= '\u202A' && r <= '\u202E') || (r >= '\u2066' && r <= '\u2069')
}

func existingDownloadError(dest string) error {
	if info, err := os.Lstat(dest); err == nil && info.IsDir() {
		return fmt.Errorf("%q already exists and is a folder; pass --output to save elsewhere", dest)
	}
	return fmt.Errorf("%q already exists; pass --force to replace it or --output to save elsewhere", dest)
}

// link is os.Link outside tests; it is a parameter so a test can make the name
// appear at the last moment, which is the case the hard link exists for.
func saveStorageDownload(dest string, body io.Reader, replace bool, link func(oldname, newname string) error) (int64, error) {
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
	if replace {
		if err := os.Rename(out.Name(), dest); err != nil {
			return 0, fmt.Errorf("save download to %s: %w", dest, err)
		}
		return written, nil
	}
	// A hard link fails when anything already has the name and never follows a
	// symlink there, so nothing can appear between a check and the write. A
	// filesystem without hard links refuses too; --force renames instead.
	if err := link(out.Name(), dest); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return 0, existingDownloadError(dest)
		}
		return 0, fmt.Errorf("save download to %q without replacing anything (pass --force or --output): %w", dest, err)
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
				confirmed, err := confirmForceable(fmt.Sprintf("Delete %s?", label))
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

// The legacy storage listing names its page size per_page, unlike the rest of
// the API.
const storagePerPageQuery = "per_page"

var storageFileLookup = lookup[storageFile]{
	kind:     "file",
	wanted:   "a file name or ID",
	listHint: "Run 'dcs storage ls' to see your files.",
	name:     func(f storageFile) string { return f.Name },
	id:       func(f storageFile) string { return f.ID },
}

func resolveStorageFile(nameOrID string) (*storageFile, error) {
	return storageFileLookup.find(context.Background(), nameOrID, func(ctx context.Context) ([]storageFile, error) {
		return listPages(ctx, client.PathStorageFiles, storagePerPageQuery, decodeStorageFiles)
	})
}

func decodeStorageFiles(resp *client.APIResponse) ([]storageFile, error) {
	listing, err := client.Decode[storageListing](resp)
	if err != nil {
		return nil, fmt.Errorf("decode storage listing: %w", err)
	}
	return listing.Files, nil
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

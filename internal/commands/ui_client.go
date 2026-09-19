package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/workspace"
)

const uiPageSize = 25

type uiSource struct{ api *client.Client }

// These projections match the API's camelCase response DTOs and deliberately
// omit credentials, deployment tokens, and other fields not shown in the UI.
type uiServer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Provider  string `json:"provider"`
	Region    string `json:"region"`
	Size      string `json:"size"`
	Tier      string `json:"tier"`
	SizeLabel string `json:"sizeLabel"`
	IPv4      string `json:"ipv4"`
	CreatedAt string `json:"createdAt"`
}

type uiSite struct {
	ID          string `json:"id"`
	Domain      string `json:"domain"`
	ProjectType string `json:"projectType"`
	Status      string `json:"status"`
	SSLEnabled  *bool  `json:"sslEnabled"`
	GitBranch   string `json:"gitBranch"`
	CreatedAt   string `json:"createdAt"`
}

func (s *uiSource) Identify(ctx context.Context) (string, error) {
	resp, err := s.get(ctx, client.PathAuthMe, nil)
	if err != nil {
		return "", fmt.Errorf("identify account: %w", err)
	}
	user, err := decodeAuthenticatedUser(resp)
	if err != nil {
		return "", fmt.Errorf("identify account: %w", safeUIError(err))
	}
	if strings.TrimSpace(user.Name) == "" {
		return user.Email, nil
	}
	return user.Name + " · " + user.Email, nil
}

func (s *uiSource) Load(ctx context.Context, req workspace.Request) (workspace.Item, error) {
	var item workspace.Item
	var err error
	switch req.Kind {
	case workspace.Servers, workspace.SiteServers:
		item, err = s.servers(ctx, req)
	case workspace.Sites:
		item, err = s.sites(ctx, req)
	case workspace.ServerDetail:
		item, err = s.server(ctx, req)
	case workspace.SiteDetail:
		item, err = s.site(ctx, req)
	default:
		err = fmt.Errorf("unsupported workspace request")
	}
	if err != nil {
		return workspace.Item{}, fmt.Errorf("load workspace: %w", safeUIError(err))
	}
	item.Request = &req
	return item, nil
}

func (s *uiSource) get(ctx context.Context, path string, query url.Values) (*client.APIResponse, error) {
	if s.api == nil {
		return nil, fmt.Errorf("read account: %w", client.ErrNotAuthenticated)
	}
	resp, err := s.api.Get(ctx, path, query)
	if err != nil {
		return nil, fmt.Errorf("read API: %w", err)
	}
	return resp, nil
}

func (s *uiSource) servers(ctx context.Context, req workspace.Request) (workspace.Item, error) {
	resp, err := s.get(ctx, client.PathServers, uiPageQuery(req))
	if err != nil {
		return workspace.Item{}, fmt.Errorf("list servers: %w", err)
	}
	servers, err := client.Decode[[]uiServer](resp)
	if err != nil {
		return workspace.Item{}, fmt.Errorf("decode servers: %w", err)
	}
	item := workspace.Item{ID: "live-servers", Title: "Your servers", Description: "Select a server to view its details.", Body: "No servers on this page. Create one with dcs servers create."}
	if req.Kind == workspace.SiteServers {
		item.ID, item.Title, item.Description = "live-sites", "Choose a server", "Select a server to browse its sites."
	}
	for _, server := range servers {
		if !validUIResourceID(server.ID) {
			return workspace.Item{}, fmt.Errorf("server response contains an invalid ID")
		}
		next := workspace.Request{Kind: workspace.ServerDetail, ServerID: server.ID}
		if req.Kind == workspace.SiteServers {
			next.Kind = workspace.Sites
		}
		item.Children = append(item.Children, workspace.Item{ID: server.ID, Title: firstUIText(server.Name, server.ID), Description: joinUIText(server.Status, server.Provider, server.Region),
			Fields: uiServerFields(server), Command: "dcs servers info -- " + workspace.Quote(server.ID), Request: &next})
	}
	return uiPaginate(item, resp.Meta, req), nil
}

func (s *uiSource) sites(ctx context.Context, req workspace.Request) (workspace.Item, error) {
	if !validUIResourceID(req.ServerID) {
		return workspace.Item{}, fmt.Errorf("invalid server ID")
	}
	resp, err := s.get(ctx, client.PathSites(url.PathEscape(req.ServerID)), uiPageQuery(req))
	if err != nil {
		return workspace.Item{}, fmt.Errorf("list sites: %w", err)
	}
	sites, err := client.Decode[[]uiSite](resp)
	if err != nil {
		return workspace.Item{}, fmt.Errorf("decode sites: %w", err)
	}
	item := workspace.Item{ID: "sites:" + req.ServerID, Title: "Sites", Description: "Select a site to view its details.", Body: "No sites on this page. Use dcs sites create --server " + workspace.Quote(req.ServerID) + " to add one."}
	for _, site := range sites {
		if !validUIResourceID(site.ID) {
			return workspace.Item{}, fmt.Errorf("site response contains an invalid ID")
		}
		next := workspace.Request{Kind: workspace.SiteDetail, ServerID: req.ServerID, SiteID: site.ID}
		item.Children = append(item.Children, workspace.Item{ID: site.ID, Title: firstUIText(site.Domain, site.ID), Description: joinUIText(site.ProjectType, site.Status), Fields: uiSiteFields(site), Request: &next})
	}
	return uiPaginate(item, resp.Meta, req), nil
}

func (s *uiSource) server(ctx context.Context, req workspace.Request) (workspace.Item, error) {
	if !validUIResourceID(req.ServerID) {
		return workspace.Item{}, fmt.Errorf("invalid server ID")
	}
	resp, err := s.get(ctx, client.PathServer(url.PathEscape(req.ServerID)), nil)
	if err != nil {
		return workspace.Item{}, fmt.Errorf("get server: %w", err)
	}
	server, err := client.Decode[uiServer](resp)
	if err != nil {
		return workspace.Item{}, fmt.Errorf("decode server: %w", err)
	}
	if server.ID != req.ServerID {
		return workspace.Item{}, fmt.Errorf("server response does not match requested ID")
	}
	return workspace.Item{ID: server.ID, Title: firstUIText(server.Name, server.ID), Description: joinUIText(server.Status, server.Provider, server.Region),
		Fields: uiServerFields(server), Command: "dcs servers info -- " + workspace.Quote(server.ID),
		Body: "Inspect only. No server changes are performed here."}, nil
}

func (s *uiSource) site(ctx context.Context, req workspace.Request) (workspace.Item, error) {
	if !validUIResourceID(req.ServerID) || !validUIResourceID(req.SiteID) {
		return workspace.Item{}, fmt.Errorf("invalid site or server ID")
	}
	resp, err := s.get(ctx, client.PathSite(url.PathEscape(req.ServerID), url.PathEscape(req.SiteID)), nil)
	if err != nil {
		return workspace.Item{}, fmt.Errorf("get site: %w", err)
	}
	site, err := client.Decode[uiSite](resp)
	if err != nil {
		return workspace.Item{}, fmt.Errorf("decode site: %w", err)
	}
	if site.ID != req.SiteID {
		return workspace.Item{}, fmt.Errorf("site response does not match requested ID")
	}
	return workspace.Item{ID: site.ID, Title: firstUIText(site.Domain, site.ID), Description: joinUIText(site.ProjectType, site.Status), Fields: uiSiteFields(site),
		Command: "dcs sites info --server " + workspace.Quote(req.ServerID) + " -- " + workspace.Quote(site.Domain),
		Body:    "Inspect only. Find deployment commands under Sites & deploys."}, nil
}

func uiServerFields(server uiServer) []workspace.Field {
	return presentUIFields([]workspace.Field{{Label: "ID", Value: server.ID}, {Label: "Status", Value: server.Status}, {Label: "Provider", Value: server.Provider}, {Label: "Region", Value: server.Region},
		{Label: "Size", Value: firstUIText(server.SizeLabel, server.Size, server.Tier)}, {Label: "IPv4", Value: server.IPv4}, {Label: "Created", Value: server.CreatedAt}})
}

func uiSiteFields(site uiSite) []workspace.Field {
	ssl := ""
	if site.SSLEnabled != nil {
		ssl = strconv.FormatBool(*site.SSLEnabled)
	}
	return presentUIFields([]workspace.Field{{Label: "ID", Value: site.ID}, {Label: "Type", Value: site.ProjectType}, {Label: "Status", Value: site.Status}, {Label: "SSL enabled", Value: ssl}, {Label: "Git branch", Value: site.GitBranch}, {Label: "Created", Value: site.CreatedAt}})
}

func presentUIFields(fields []workspace.Field) []workspace.Field {
	var present []workspace.Field
	for _, field := range fields {
		if field.Value != "" {
			present = append(present, field)
		}
	}
	return present
}

func firstUIText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func joinUIText(values ...string) string {
	var parts []string
	for _, value := range values {
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " · ")
}

func validUIResourceID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\") && workspace.Clean(id) == id
}

func uiPageQuery(req workspace.Request) url.Values {
	return url.Values{"page": {strconv.Itoa(max(1, req.Page))}, "perPage": {strconv.Itoa(uiPageSize)}}
}

func uiPaginate(item workspace.Item, meta *client.Meta, req workspace.Request) workspace.Item {
	if meta == nil {
		return item
	}
	item.Description += fmt.Sprintf(" Page %d of %d.", max(1, meta.Page), max(1, meta.TotalPages))
	add := func(title string, page int) {
		next := req
		next.Page = page
		item.Children = append(item.Children, workspace.Item{ID: fmt.Sprintf("page:%d", page), Title: title, Description: fmt.Sprintf("Read page %d.", page), Request: &next})
	}
	if meta.Page > 1 {
		add("Previous page", meta.Page-1)
	}
	if meta.Page < meta.TotalPages {
		add("Next page", meta.Page+1)
	}
	return item
}

type uiReadError struct {
	cause   error
	message string
}

func (e *uiReadError) Error() string { return e.message }
func (e *uiReadError) Unwrap() error { return e.cause }

// Raw transport URLs and API error bodies can contain secrets. Keep them out
// of terminal rendering while retaining the cause for errors.Is and tests.
func safeUIError(err error) error {
	message := "Request failed. Check your connection and dcs whoami, then retry."
	switch {
	case errors.Is(err, client.ErrNotAuthenticated):
		message = "Sign in with dcs login, then reopen dcs ui."
	case errors.Is(err, client.ErrForbidden):
		message = "Your API key cannot read this resource. Check its permissions."
	case errors.Is(err, client.ErrRateLimited):
		message = "Too many requests. Wait a moment, then retry."
	case errors.Is(err, context.DeadlineExceeded):
		message = "The request timed out. Check your connection, then retry."
	case errors.Is(err, context.Canceled):
		message = "Request canceled."
	}
	return &uiReadError{cause: err, message: message}
}

package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
)

// completeServerNames returns server names for shell completion.
func completeServerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if apiClient == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	resp, err := apiClient.Get(context.Background(), client.PathServers, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	servers, err := client.Decode[[]Server](resp)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, len(servers))
	for i, s := range servers {
		names[i] = s.Name
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeSiteNames returns site domains for shell completion.
// It requires a --server flag to resolve the server first.
func completeSiteNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if apiClient == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	serverFlag, _ := cmd.Flags().GetString("server")
	serverID, err := getServerID(serverFlag)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	resp, err := apiClient.Get(context.Background(), client.PathSites(serverID), nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	sites, err := client.Decode[[]Site](resp)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	domains := make([]string, len(sites))
	for i, s := range sites {
		domains[i] = s.Domain
	}
	return domains, cobra.ShellCompDirectiveNoFileComp
}

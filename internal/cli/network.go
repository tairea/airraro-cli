// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: Air Rarotonga route graph.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type networkAirport struct {
	Name        string       `json:"name"`
	Code        string       `json:"code"`
	Connections []airportRef `json:"connections"`
}

type networkResponse struct {
	Airports []networkAirport `json:"airports"`
}

type networkNode struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Connections []string `json:"connections"`
}

// pp:data-source live
func newNovelNetworkCmd(flags *rootFlags) *cobra.Command {
	var from string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Full Cook Islands route graph: every airport and where it connects.",
		Long: "Show Air Rarotonga's route network as a graph of origin airports and the destinations\n" +
			"each one connects to. Use --from to show connections for a single airport.\n" +
			"Example: airraro-pp-cli network --from RAR --agent",
		Example:     "  airraro-pp-cli network --from RAR --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch route network")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/Airport/OriginsWithConnections/en-us", nil)
			if err != nil {
				return fmt.Errorf("fetching route network: %w", err)
			}
			var resp networkResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("decoding route network: %w", err)
			}

			nodes := make([]networkNode, 0, len(resp.Airports))
			filter := upperCode(from)
			for _, a := range resp.Airports {
				if filter != "" && a.Code != filter {
					continue
				}
				conns := make([]string, 0, len(a.Connections))
				for _, c := range a.Connections {
					conns = append(conns, c.Code)
				}
				sort.Strings(conns)
				nodes = append(nodes, networkNode{Code: a.Code, Name: a.Name, Connections: conns})
			}
			sort.Slice(nodes, func(i, j int) bool { return nodes[i].Code < nodes[j].Code })

			if filter != "" && len(nodes) == 0 {
				return usageErr(fmt.Errorf("unknown airport code %q (try `airraro-pp-cli network` to list all)", filter))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range nodes {
					fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) -> %d: %v\n", n.Code, n.Name, len(n.Connections), n.Connections)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), nodes, flags)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Show connections for a single airport code (e.g. RAR)")
	return cmd
}

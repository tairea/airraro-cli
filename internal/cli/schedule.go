// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: the full weekly schedule grid (all flights),
// extracted from Air Rarotonga's published "Aircraft Allocation and Weekly
// Schedule" Google Sheet. Built on the same span-aware grid parser as `freight`
// (see freight.go); `freight` is the green-font subset of this command.

package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type scheduleView struct {
	Source    string            `json:"source"`
	FetchedAt string            `json:"fetched_at"`
	Filters   map[string]string `json:"filters,omitempty"`
	Count     int               `json:"count"`
	Flights   []scheduleEntry   `json:"flights"`
}

// pp:data-source live
func newNovelScheduleCmd(flags *rootFlags) *cobra.Command {
	var island, day, category, date, url string
	var week, limit int

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "List the full weekly flight schedule from the aircraft-allocation sheet.",
		Long: "Extract every flight from Air Rarotonga's published \"Aircraft Allocation and Weekly\n" +
			"Schedule\" Google Sheet — all islands, all days, across the rolling multi-week window.\n" +
			"Each flight carries week, date, day, departure time, route, and a colour-derived category\n" +
			"(freight = green font; scheduled = everything else, with the raw font colour preserved).\n" +
			"Filter with --island, --day, --week, --date, or --category.\n" +
			"Example: airraro-pp-cli schedule --island MUK --day THU --agent",
		Example:     "  airraro-pp-cli schedule --island AIU --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and parse the full weekly aircraft-allocation schedule grid")
				return nil
			}

			src := url
			if src == "" {
				src = freightSheetURL
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			html, err := fetchFreightSheet(ctx, src)
			if err != nil {
				return apiErr(fmt.Errorf("fetching schedule sheet: %w", err))
			}
			flights, err := parseScheduleEntries(html)
			if err != nil {
				return apiErr(fmt.Errorf("parsing schedule sheet: %w", err))
			}

			islandF := upperCode(island)
			dayF := normalizeDay(day)
			catF := strings.ToLower(strings.TrimSpace(category))
			dateF := strings.TrimSpace(date)

			filters := map[string]string{}
			if islandF != "" {
				filters["island"] = islandF
			}
			if dayF != "" {
				filters["day"] = dayF
			}
			if week > 0 {
				filters["week"] = strconv.Itoa(week)
			}
			if dateF != "" {
				filters["date"] = dateF
			}
			if catF != "" {
				filters["category"] = catF
			}

			kept := flights[:0:0]
			for _, f := range flights {
				if islandF != "" && !containsCode(f.Islands, islandF) {
					continue
				}
				if dayF != "" && f.Day != dayF {
					continue
				}
				if week > 0 && f.Week != week {
					continue
				}
				if dateF != "" && f.Date != dateF {
					continue
				}
				if catF != "" && f.Category != catF {
					continue
				}
				kept = append(kept, f)
			}
			if limit > 0 && len(kept) > limit {
				kept = kept[:limit]
			}

			view := scheduleView{
				Source:    src,
				FetchedAt: time.Now().UTC().Format(time.RFC3339),
				Filters:   filters,
				Count:     len(kept),
				Flights:   kept,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderScheduleTable(cmd.OutOrStdout(), view)
		},
	}

	cmd.Flags().StringVarP(&island, "island", "i", "", "filter to a destination IATA code (e.g. MUK, AIU)")
	cmd.Flags().StringVarP(&day, "day", "d", "", "filter to a weekday (MON..SUN)")
	cmd.Flags().IntVarP(&week, "week", "w", 0, "filter to a week number (e.g. 25)")
	cmd.Flags().StringVar(&date, "date", "", "filter to an ISO date (yyyy-mm-dd)")
	cmd.Flags().StringVarP(&category, "category", "c", "", "filter to a category (freight|scheduled)")
	cmd.Flags().StringVar(&url, "url", "", "override the published schedule sheet URL")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum flights to return (0 = all)")
	return cmd
}

func renderScheduleTable(w io.Writer, v scheduleView) error {
	if v.Count == 0 {
		fmt.Fprintln(w, "No flights matched.")
		return nil
	}
	fmt.Fprintf(w, "%d flight(s) from the weekly aircraft-allocation sheet:\n\n", v.Count)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "WEEK\tDATE\tDAY\tTIME\tROUTE\tCATEGORY")
	for _, f := range v.Flights {
		week := ""
		if f.Week > 0 {
			week = strconv.Itoa(f.Week)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", week, f.Date, f.Day, f.Time, f.Route, f.Category)
	}
	return tw.Flush()
}

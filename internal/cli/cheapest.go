// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: cheapest fare per day across a window.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type cheapestDay struct {
	Date     string  `json:"date"`
	Flight   string  `json:"flight"`
	Depart   string  `json:"depart"`
	Arrive   string  `json:"arrive"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

type cheapestView struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	Currency string        `json:"currency"`
	Days     []cheapestDay `json:"days"`
	Cheapest *cheapestDay  `json:"cheapest,omitempty"`
}

// pp:data-source live
func newNovelCheapestCmd(flags *rootFlags) *cobra.Command {
	var date string
	var window int
	var currency string
	var adults, children, infants int

	cmd := &cobra.Command{
		Use:   "cheapest <from> <to>",
		Short: "Find the lowest fare per day across a date window for a route.",
		Long: "Find the lowest available fare for each day across a date window for an Air Rarotonga route.\n" +
			"One SearchShop call returns a window of fares; this command ranks the cheapest option per day.\n" +
			"Example: airraro-pp-cli cheapest RAR AIT --date 2026-07-15 --window 7 --agent",
		Example:     "  airraro-pp-cli cheapest RAR AIT --date 2026-07-15 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search cheapest fares per day")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two positional arguments required: <from> <to> airport codes"))
			}
			from, to := upperCode(args[0]), upperCode(args[1])

			if date == "" {
				date = time.Now().AddDate(0, 0, 30).Format("2006-01-02")
			}
			target, err := normDate(date)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--date must be YYYY-MM-DD: %w", err))
			}
			if window < 1 {
				window = 1
			}
			start := target.AddDate(0, 0, -window).Format("2006-01-02")
			end := target.AddDate(0, 0, window).Format("2006-01-02")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			resp, err := runSearchShop(ctx, flags, from, to, date, start, end, currency, adults, children, infants)
			if err != nil {
				return err
			}

			best := map[string]cheapestDay{}
			for _, route := range resp.Routes {
				for _, fl := range route.Flights {
					price, ok := priceFloat(fl.LowestPriceTotal)
					if !ok || price <= 0 {
						continue
					}
					day := flightDay(fl.DepartureDate)
					cur, exists := best[day]
					if !exists || price < cur.Price {
						best[day] = cheapestDay{
							Date:     day,
							Flight:   fl.CarrierCode + fl.FlightNumber,
							Depart:   flightTime(fl.DepartureDate),
							Arrive:   flightTime(fl.ArrivalDate),
							Price:    price,
							Currency: currency,
						}
					}
				}
			}

			view := cheapestView{From: from, To: to, Currency: currency, Days: make([]cheapestDay, 0, len(best))}
			for _, d := range best {
				view.Days = append(view.Days, d)
			}
			sort.Slice(view.Days, func(i, j int) bool { return view.Days[i].Date < view.Days[j].Date })
			for i := range view.Days {
				if view.Cheapest == nil || view.Days[i].Price < view.Cheapest.Price {
					d := view.Days[i]
					view.Cheapest = &d
				}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(view.Days) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No fares found for %s->%s near %s\n", from, to, date)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s (cheapest per day, %s)\n", from, to, currency)
				for _, d := range view.Days {
					marker := "  "
					if view.Cheapest != nil && d.Date == view.Cheapest.Date && d.Price == view.Cheapest.Price {
						marker = "* "
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s%s  %-7s %s->%s  %s %.2f\n", marker, d.Date, d.Flight, d.Depart, d.Arrive, currency, d.Price)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Target departure date YYYY-MM-DD (default ~30 days out)")
	cmd.Flags().IntVar(&window, "window", 7, "Days before/after the target date to scan")
	cmd.Flags().StringVar(&currency, "currency", "NZD", "Currency code")
	cmd.Flags().IntVar(&adults, "adults", 1, "Number of adult passengers")
	cmd.Flags().IntVar(&children, "children", 0, "Number of child passengers")
	cmd.Flags().IntVar(&infants, "infants", 0, "Number of infant passengers")
	return cmd
}

// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: fare calendar across a date window.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type calendarFlight struct {
	Flight   string  `json:"flight"`
	Depart   string  `json:"depart"`
	Arrive   string  `json:"arrive"`
	Cabin    string  `json:"cabin"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

type calendarDay struct {
	Date    string           `json:"date"`
	Lowest  float64          `json:"lowest"`
	Flights []calendarFlight `json:"flights"`
}

type calendarView struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	Currency string        `json:"currency"`
	Days     []calendarDay `json:"days"`
}

// pp:data-source live
func newNovelFareCalendarCmd(flags *rootFlags) *cobra.Command {
	var date string
	var days int
	var currency string
	var adults, children, infants int

	cmd := &cobra.Command{
		Use:   "fare-calendar <from> <to>",
		Short: "Fares for a route across many dates.",
		Long: "Show every Air Rarotonga flight and its lowest fare for each day across a date window.\n" +
			"Example: airraro-pp-cli fare-calendar RAR AIT --date 2026-07-15 --days 14 --agent",
		Example:     "  airraro-pp-cli fare-calendar RAR AIT --days 14 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch fare calendar")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two positional arguments required: <from> <to> airport codes"))
			}
			from, to := upperCode(args[0]), upperCode(args[1])

			if date == "" {
				date = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
			}
			target, err := normDate(date)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--date must be YYYY-MM-DD: %w", err))
			}
			if days < 1 {
				days = 1
			}
			start := target.Format("2006-01-02")
			end := target.AddDate(0, 0, days).Format("2006-01-02")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			resp, err := runSearchShop(ctx, flags, from, to, date, start, end, currency, adults, children, infants)
			if err != nil {
				return err
			}

			byDay := map[string]*calendarDay{}
			for _, route := range resp.Routes {
				for _, fl := range route.Flights {
					price, ok := priceFloat(fl.LowestPriceTotal)
					if !ok || price <= 0 {
						continue
					}
					day := flightDay(fl.DepartureDate)
					cd, exists := byDay[day]
					if !exists {
						cd = &calendarDay{Date: day, Lowest: price}
						byDay[day] = cd
					}
					if price < cd.Lowest {
						cd.Lowest = price
					}
					cd.Flights = append(cd.Flights, calendarFlight{
						Flight:   fl.CarrierCode + fl.FlightNumber,
						Depart:   flightTime(fl.DepartureDate),
						Arrive:   flightTime(fl.ArrivalDate),
						Cabin:    fl.Cabin,
						Price:    price,
						Currency: currency,
					})
				}
			}

			view := calendarView{From: from, To: to, Currency: currency, Days: make([]calendarDay, 0, len(byDay))}
			for _, cd := range byDay {
				sort.Slice(cd.Flights, func(i, j int) bool { return cd.Flights[i].Depart < cd.Flights[j].Depart })
				view.Days = append(view.Days, *cd)
			}
			sort.Slice(view.Days, func(i, j int) bool { return view.Days[i].Date < view.Days[j].Date })

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(view.Days) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No fares found for %s->%s from %s\n", from, to, start)
					return nil
				}
				for _, d := range view.Days {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  (lowest %s %.2f)\n", d.Date, currency, d.Lowest)
					for _, f := range d.Flights {
						fmt.Fprintf(cmd.OutOrStdout(), "    %-7s %s->%s  %-6s %s %.2f\n", f.Flight, f.Depart, f.Arrive, f.Cabin, currency, f.Price)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Start date YYYY-MM-DD (default ~7 days out)")
	cmd.Flags().IntVar(&days, "days", 14, "Number of days forward to include")
	cmd.Flags().StringVar(&currency, "currency", "NZD", "Currency code")
	cmd.Flags().IntVar(&adults, "adults", 1, "Number of adult passengers")
	cmd.Flags().IntVar(&children, "children", 0, "Number of child passengers")
	cmd.Flags().IntVar(&infants, "infants", 0, "Number of infant passengers")
	return cmd
}

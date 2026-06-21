// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored helper for Air Rarotonga flight search (Sabre EzyCommerce SearchShop).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// searchShopResponse mirrors the fields of POST /Availability/SearchShop the
// CLI consumes. The upstream payload is large; we decode only what commands use.
type searchShopResponse struct {
	Routes []struct {
		From    airportRef  `json:"from"`
		To      airportRef  `json:"to"`
		Flights []shopFlight `json:"flights"`
	} `json:"routes"`
}

type airportRef struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type shopFlight struct {
	CarrierCode      string      `json:"carrierCode"`
	FlightNumber     string      `json:"flightNumber"`
	DepartureDate    string      `json:"departureDate"`
	ArrivalDate      string      `json:"arrivalDate"`
	Cabin            string      `json:"cabin"`
	LowestPriceTotal json.Number `json:"lowestPriceTotal"`
	Fares            []shopFare  `json:"fares"`
}

type shopFare struct {
	Name      string      `json:"name"`
	FareBasis string      `json:"fareBasis"`
	Code      string      `json:"code"`
	Price     json.Number `json:"price"`
}

// runSearchShop builds the SearchShop request body for a single one-way route
// over [startDate, endDate] and returns the decoded response. departureDate is
// the target day; startDate/endDate bound the window the API returns fares for.
func runSearchShop(ctx context.Context, flags *rootFlags, from, to, departureDate, startDate, endDate, currency string, adults, children, infants int) (*searchShopResponse, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"passengers": []map[string]any{
			{"code": "ADT", "count": adults},
			{"code": "CHD", "count": children},
			{"code": "INF", "count": infants},
		},
		"routes": []map[string]any{
			{
				"fromAirport":   from,
				"toAirport":     to,
				"departureDate": departureDate,
				"startDate":     startDate,
				"endDate":       endDate,
			},
		},
		"currency":           currency,
		"fareTypeCategories": nil,
		"isManageBooking":    false,
		"languageCode":       "en-us",
	}
	data, status, err := c.Post(ctx, "/Availability/SearchShop", body)
	if err != nil {
		return nil, fmt.Errorf("SearchShop request failed (HTTP %d): %w", status, err)
	}
	// A 2xx SearchShop response may still carry a partial-failure envelope.
	// Honor the global --allow-partial-failure flag: surface a typed
	// partial-failure error (exit 6) unless the caller opted to downgrade it.
	if report := detectPartialFailure(data); report != nil && !flags.allowPartialFailure {
		return nil, partialFailureErr(fmt.Errorf("SearchShop reported a partial failure on %q: %s", report.Field, report.Message))
	}
	var resp searchShopResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decoding SearchShop response: %w", err)
	}
	return &resp, nil
}

// flightDay extracts the YYYY-MM-DD portion of an upstream datetime string
// like "2026-07-15T13:30:00".
func flightDay(dt string) string {
	if i := strings.IndexByte(dt, 'T'); i > 0 {
		return dt[:i]
	}
	return dt
}

// flightTime extracts HH:MM from an upstream datetime string.
func flightTime(dt string) string {
	if i := strings.IndexByte(dt, 'T'); i > 0 && len(dt) >= i+6 {
		return dt[i+1 : i+6]
	}
	return ""
}

// priceFloat parses an upstream json.Number price; returns 0 and false when empty.
func priceFloat(n json.Number) (float64, bool) {
	if n == "" {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}

// normDate validates and normalizes a YYYY-MM-DD date string.
func normDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// upperCode normalizes an airport code argument.
func upperCode(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

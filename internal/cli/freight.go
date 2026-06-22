// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: extract green-font freight runs from Air
// Rarotonga's published "Aircraft Allocation and Weekly Schedule" Google Sheet.
//
// Why this is its own data source: the freight/charter runs (Mauke, Mangaia,
// Atiu, Aitutaki) are flagged ONLY by green font colour (#00b050) in the
// published sheet grid. That signal exists in the published HTML
// (pubhtml/sheet) but is destroyed by the CSV export, so this parser reads the
// HTML, resolves which CSS classes carry the green font, and expands the
// colspan/rowspan grid into a dense matrix to attach week/date/day/time to
// each green cell.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// freightSheetURL is the published-to-web ("pubhtml") rendering of the
// "Aircraft Allocation and Weekly Schedule" tab linked from
// https://airraro.com/schedules/. The /pubhtml/sheet sub-endpoint returns the
// rendered grid with per-cell CSS classes; the gid pins the schedule tab.
const freightSheetURL = "https://docs.google.com/spreadsheets/d/e/2PACX-1vTlFCFNkNFE0p50Xh_jwjDWddhlNEX7jCk4-mPGbdx1mKOloEaDeHP4KpvXaqf-DvUOq4h9bZTr8atL/pubhtml/sheet?gid=350413081"

// freightGreenFont is the Google Sheets standard-green font colour used to mark
// freight runs. Cells whose CSS class sets this as the FONT colour (not the
// background) are freight.
const freightGreenFont = "#00b050"

// blockWidth is the column stride of one week-block in the grid: a time/label
// column, seven day columns (MON..SUN), and a separator column.
const blockWidth = 9

type freightRun struct {
	Week    int      `json:"week,omitempty"`
	Date    string   `json:"date,omitempty"` // ISO yyyy-mm-dd, best-effort
	Day     string   `json:"day,omitempty"`  // MON..SUN
	Time    string   `json:"time,omitempty"` // local departure, e.g. 0900
	Route   string   `json:"route"`          // raw cell text, e.g. MUK, AIU-MUK
	Islands []string `json:"islands"`        // 3-letter codes found in route
	Freight bool     `json:"freight"`        // always true (green-font flagged)
}

type freightView struct {
	Source    string       `json:"source"`
	FetchedAt string       `json:"fetched_at"`
	Filter    string       `json:"island_filter,omitempty"`
	Count     int          `json:"count"`
	Runs      []freightRun `json:"runs"`
}

// pp:data-source live
func newNovelFreightCmd(flags *rootFlags) *cobra.Command {
	var island string
	var url string
	var limit int
	var debug bool

	cmd := &cobra.Command{
		Use:   "freight",
		Short: "List green-font freight runs from the weekly aircraft-allocation sheet.",
		Long: "Extract the freight/charter runs flagged in green font on Air Rarotonga's published\n" +
			"\"Aircraft Allocation and Weekly Schedule\" Google Sheet (Mauke, Mangaia, Atiu, Aitutaki).\n" +
			"The green-font freight signal exists only in the published HTML, not the CSV export, so this\n" +
			"command reads the rendered grid directly. Use --island MUK to narrow to Mauke freight runs.\n" +
			"Example: airraro-pp-cli freight --island MUK --agent",
		Example:     "  airraro-pp-cli freight --island MUK --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and parse the weekly aircraft-allocation sheet for green-font freight runs")
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

			if debug {
				dumpFreightGrid(cmd.OutOrStdout(), html)
				return nil
			}

			runs, err := parseFreightRuns(html)
			if err != nil {
				return apiErr(fmt.Errorf("parsing schedule sheet: %w", err))
			}

			filter := upperCode(island)
			if filter != "" {
				kept := runs[:0:0]
				for _, r := range runs {
					if containsCode(r.Islands, filter) {
						kept = append(kept, r)
					}
				}
				runs = kept
			}

			if limit > 0 && len(runs) > limit {
				runs = runs[:limit]
			}

			view := freightView{
				Source:    src,
				FetchedAt: time.Now().UTC().Format(time.RFC3339),
				Filter:    filter,
				Count:     len(runs),
				Runs:      runs,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderFreightTable(cmd.OutOrStdout(), view)
		},
	}

	cmd.Flags().StringVarP(&island, "island", "i", "", "filter to a destination IATA code (e.g. MUK for Mauke)")
	cmd.Flags().StringVar(&url, "url", "", "override the published schedule sheet URL")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum freight runs to return (0 = all)")
	cmd.Flags().BoolVar(&debug, "debug-grid", false, "dump the parsed dense grid for diagnostics")
	_ = cmd.Flags().MarkHidden("debug-grid")
	return cmd
}

func fetchFreightSheet(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	// A browser-ish UA avoids occasional Google interstitials for published sheets.
	req.Header.Set("User-Agent", "airraro-pp-cli/freight (+https://airraro.com/schedules)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// --- grid parsing ---------------------------------------------------------

type gridCell struct {
	class string
	text  string
}

var (
	styleRuleRe = regexp.MustCompile(`\.(s\d+)\s*\{([^}]*)\}`)
	trRe        = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	tdRe        = regexp.MustCompile(`(?is)<td([^>]*)>(.*?)</td>`)
	classAttrRe = regexp.MustCompile(`class="([^"]*)"`)
	colspanRe   = regexp.MustCompile(`colspan="(\d+)"`)
	rowspanRe   = regexp.MustCompile(`rowspan="(\d+)"`)
	tagRe       = regexp.MustCompile(`(?is)<[^>]*>`)
	codeRe      = regexp.MustCompile(`[A-Z]{3}`)
	dayNameRe   = regexp.MustCompile(`^(MON|TUE|WED|THUR|THU|FRI|SAT|SUN)$`)
	fullDateRe  = regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{4})`)
)

// cssColorByClass maps each CSS class to the value of the requested colour
// property ("color" for font, "background-color" for fill). Resolving by colour
// rather than hardcoded class ids keeps the parser working when the sheet is
// re-published and class numbering changes.
func cssColorByClass(html, prop string) map[string]string {
	out := map[string]string{}
	prefix := prop + ":"
	for _, m := range styleRuleRe.FindAllStringSubmatch(html, -1) {
		class, body := m[1], strings.ToLower(m[2])
		for _, decl := range strings.Split(body, ";") {
			decl = strings.TrimSpace(decl)
			if strings.HasPrefix(decl, prefix) {
				out[class] = strings.TrimSpace(strings.TrimPrefix(decl, prefix))
			}
		}
	}
	return out
}

func fontColorByClass(html string) map[string]string { return cssColorByClass(html, "color") }
func bgColorByClass(html string) map[string]string {
	return cssColorByClass(html, "background-color")
}

// greenFontClasses returns the set of CSS class names whose rule sets the
// freight green as the FONT colour (background green does not count).
func greenFontClasses(html string) map[string]bool {
	out := map[string]bool{}
	for class, v := range fontColorByClass(html) {
		if v == freightGreenFont {
			out[class] = true
		}
	}
	return out
}

// denseGrid expands the colspan/rowspan table into a rectangular matrix so that
// column indices are stable across header and data rows.
func denseGrid(html string) [][]gridCell {
	rows := trRe.FindAllStringSubmatch(html, -1)
	var grid [][]gridCell
	// pending tracks cells still spanning down into future rows, keyed by col.
	type span struct {
		rowsLeft int
		cell     gridCell
	}
	pending := map[int]*span{}

	for _, row := range rows {
		var line []gridCell
		col := 0
		place := func(c gridCell) {
			for len(line) <= col {
				line = append(line, gridCell{})
			}
			line[col] = c
		}
		emitPending := func() {
			for {
				p, ok := pending[col]
				if !ok {
					return
				}
				place(p.cell)
				p.rowsLeft--
				if p.rowsLeft <= 0 {
					delete(pending, col)
				}
				col++
			}
		}

		emitPending()
		for _, td := range tdRe.FindAllStringSubmatch(row[1], -1) {
			attrs, inner := td[1], td[2]
			class := ""
			if m := classAttrRe.FindStringSubmatch(attrs); m != nil {
				class = m[1]
			}
			colspan := 1
			if m := colspanRe.FindStringSubmatch(attrs); m != nil {
				colspan, _ = strconv.Atoi(m[1])
			}
			rowspan := 1
			if m := rowspanRe.FindStringSubmatch(attrs); m != nil {
				rowspan, _ = strconv.Atoi(m[1])
			}
			text := cellText(inner)
			cell := gridCell{class: class, text: text}
			for k := 0; k < colspan; k++ {
				place(cell)
				if rowspan > 1 {
					pending[col] = &span{rowsLeft: rowspan - 1, cell: cell}
				}
				col++
				emitPending()
			}
		}
		grid = append(grid, line)
	}
	return grid
}

func cellText(inner string) string {
	t := tagRe.ReplaceAllString(inner, "")
	t = strings.ReplaceAll(t, "&amp;", "&")
	t = strings.ReplaceAll(t, "&nbsp;", " ")
	t = strings.ReplaceAll(t, "&#39;", "'")
	return strings.TrimSpace(t)
}

// scheduleEntry is one flight cell from the grid: a day-column cell carrying a
// route token, with week/date/day/time attached and a font-colour-derived
// category. Category is "freight" for green-font cells; everything else is
// "scheduled" with the raw font colour preserved in Color (the sheet ships no
// legend, so finer colour meanings are surfaced, not invented).
type scheduleEntry struct {
	Week     int      `json:"week,omitempty"`
	Date     string   `json:"date,omitempty"`
	Day      string   `json:"day,omitempty"`
	Time     string   `json:"time,omitempty"`
	Route    string   `json:"route"`
	Islands  []string `json:"islands"`
	Color    string   `json:"color,omitempty"`
	Category string   `json:"category"`
}

var routeRe = regexp.MustCompile(`^[A-Z]{2,4}(?:[-/][A-Z0-9]{2,5})*$`)

// isRouteText reports whether a day-column cell holds a flight route token
// (single IATA code or codes joined by - or /), excluding day names and the
// section markers that share the day columns.
func isRouteText(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	if u == "" || dayNameRe.MatchString(u) {
		return false
	}
	switch u {
	case "WEEK", "TO", "SMW", "EFS":
		return false
	}
	return routeRe.MatchString(u) && codeRe.MatchString(u)
}

// buildHeaderMaps resolves the per-column day name, per-column ISO date, and
// per-block week number from the day-name header row and the date row beneath it.
func buildHeaderMaps(grid [][]gridCell, dayHeaderRow, year int) (dayByCol map[int]string, dateByCol map[int]string, weekByBlock map[int]int) {
	dayByCol = map[int]string{}
	dateByCol = map[int]string{}
	weekByBlock = map[int]int{}
	if dayHeaderRow < 0 {
		return
	}
	for c, cell := range grid[dayHeaderRow] {
		if dayNameRe.MatchString(cell.text) {
			dayByCol[c] = normalizeDay(cell.text)
		}
	}
	for b := 0; b*blockWidth < len(grid[dayHeaderRow]); b++ {
		c := b * blockWidth
		if w, err := strconv.Atoi(strings.TrimSpace(grid[dayHeaderRow][c].text)); err == nil {
			weekByBlock[b] = w
		}
	}
	if dr := dayHeaderRow + 1; dr < len(grid) {
		for c, cell := range grid[dr] {
			if iso := parseGridDate(cell.text, year); iso != "" {
				dateByCol[c] = iso
			}
		}
	}
	return
}

func categoryForColor(hex string) string {
	if hex == freightGreenFont {
		return "freight"
	}
	return "scheduled"
}

// parseScheduleEntries is the shared core: it expands the grid and emits every
// day-column flight cell with week/date/day/time and a colour-derived category.
// White-background filtering drops section markers (SMW/EFS) and the olive
// charter-label blocks, which share the day columns but are not flights.
func parseScheduleEntries(html string) ([]scheduleEntry, error) {
	grid := denseGrid(html)
	if len(grid) == 0 {
		return nil, fmt.Errorf("no table rows parsed; sheet layout may have changed")
	}
	fontColor := fontColorByClass(html)
	bgColor := bgColorByClass(html)
	year := inferYear(grid)
	dhr := findDayHeaderRow(grid)
	dayByCol, dateByCol, weekByBlock := buildHeaderMaps(grid, dhr, year)

	var out []scheduleEntry
	for r, line := range grid {
		if r <= dhr+1 { // skip the header region (week/day/date rows)
			continue
		}
		for c, cell := range line {
			inblock := c % blockWidth
			if inblock < 1 || inblock > 7 { // day columns only (1..7 = MON..SUN)
				continue
			}
			if cell.text == "" || !isRouteText(cell.text) {
				continue
			}
			if bg := bgColor[cell.class]; bg != "" && bg != "#ffffff" {
				continue // marker/section header, not a flight
			}
			color := fontColor[cell.class]
			block := c / blockWidth
			out = append(out, scheduleEntry{
				Week:     weekByBlock[block],
				Date:     dateByCol[c],
				Day:      dayByCol[c],
				Time:     blockTime(line, block),
				Route:    cell.text,
				Islands:  codeRe.FindAllString(strings.ToUpper(cell.text), -1),
				Color:    color,
				Category: categoryForColor(color),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Time < out[j].Time
	})
	return out, nil
}

// parseFreightRuns returns only the green-font freight entries as freightRun
// records (back-compatible projection of the shared schedule parser).
func parseFreightRuns(html string) ([]freightRun, error) {
	entries, err := parseScheduleEntries(html)
	if err != nil {
		return nil, err
	}
	var runs []freightRun
	for _, e := range entries {
		if e.Category != "freight" {
			continue
		}
		runs = append(runs, freightRun{
			Week:    e.Week,
			Date:    e.Date,
			Day:     e.Day,
			Time:    e.Time,
			Route:   e.Route,
			Islands: e.Islands,
			Freight: true,
		})
	}
	return runs, nil
}

// blockTime returns the departure time from the leftmost (time) column of the
// given week-block on this row.
func blockTime(line []gridCell, block int) string {
	c := block * blockWidth
	if c < len(line) {
		t := strings.TrimSpace(line[c].text)
		if regexp.MustCompile(`^\d{3,4}$`).MatchString(t) {
			return t
		}
	}
	return ""
}

func findDayHeaderRow(grid [][]gridCell) int {
	for r, line := range grid {
		hits := 0
		for _, cell := range line {
			if dayNameRe.MatchString(cell.text) {
				hits++
			}
		}
		if hits >= 5 {
			return r
		}
	}
	return -1
}

func inferYear(grid [][]gridCell) int {
	for _, line := range grid {
		for _, cell := range line {
			if m := fullDateRe.FindStringSubmatch(cell.text); m != nil {
				if y, err := strconv.Atoi(m[3]); err == nil {
					return y
				}
			}
		}
	}
	return time.Now().Year()
}

// parseGridDate normalizes the date shapes seen in the sheet ("16Jun",
// "01Jul", "6/15/2026") to ISO yyyy-mm-dd. Returns "" when the text is not a date.
func parseGridDate(s string, year int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if m := fullDateRe.FindStringSubmatch(s); m != nil {
		mo, _ := strconv.Atoi(m[1])
		d, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		return fmt.Sprintf("%04d-%02d-%02d", y, mo, d)
	}
	// "16Jun" / "6Jul" / "01Jul"
	m := regexp.MustCompile(`^(\d{1,2})([A-Za-z]{3})$`).FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	d, _ := strconv.Atoi(m[1])
	mo, ok := monthNum[strings.ToLower(m[2])]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, mo, d)
}

var monthNum = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

func normalizeDay(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "THUR" {
		return "THU"
	}
	return s
}

func containsCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func renderFreightTable(w io.Writer, v freightView) error {
	if v.Count == 0 {
		fmt.Fprintln(w, "No freight runs found.")
		if v.Filter != "" {
			fmt.Fprintf(w, "(filtered to island %s; drop --island to see all freight runs)\n", v.Filter)
		}
		return nil
	}
	fmt.Fprintf(w, "%d freight run(s) from the weekly aircraft-allocation sheet:\n\n", v.Count)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "WEEK\tDATE\tDAY\tTIME\tROUTE")
	for _, r := range v.Runs {
		week := ""
		if r.Week > 0 {
			week = strconv.Itoa(r.Week)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", week, r.Date, r.Day, r.Time, r.Route)
	}
	return tw.Flush()
}

// dumpFreightGrid prints the dense grid + resolved header mapping for diagnostics.
func dumpFreightGrid(w io.Writer, html string) {
	green := greenFontClasses(html)
	fmt.Fprintf(w, "green-font classes: %v\n", green)
	grid := denseGrid(html)
	dhr := findDayHeaderRow(grid)
	fmt.Fprintf(w, "year=%d dayHeaderRow=%d rows=%d\n", inferYear(grid), dhr, len(grid))
	if dhr >= 0 {
		fmt.Fprintf(w, "dayHeader: %s\n", rowDump(grid[dhr]))
		if dhr+1 < len(grid) {
			fmt.Fprintf(w, "dateRow  : %s\n", rowDump(grid[dhr+1]))
		}
	}
	for r, line := range grid {
		for c, cell := range line {
			if cell.text != "" && green[cell.class] {
				fmt.Fprintf(w, "GREEN r%d c%d block%d time=%q day-col=%q text=%q\n",
					r, c, c/blockWidth, blockTime(line, c/blockWidth), colDayName(grid, dhr, c), cell.text)
			}
		}
	}
}

func colDayName(grid [][]gridCell, dhr, c int) string {
	if dhr >= 0 && c < len(grid[dhr]) {
		return grid[dhr][c].text
	}
	return ""
}

func rowDump(line []gridCell) string {
	var b strings.Builder
	for c, cell := range line {
		if cell.text != "" {
			fmt.Fprintf(&b, "c%d=%q ", c, cell.text)
		}
	}
	return b.String()
}

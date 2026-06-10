package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"io"
)

const (
	cbsAPI       = "https://api.cbs.gov.il/index/data/price"
	expectedBase = "average 2024"
	label        = "RG1"
	reportDay    = 15
	outFile      = "cpi.txt"
)

// ErrBaseChanged signals the CBS base differs from the expected one.
var ErrBaseChanged = errors.New("CPI base changed")

type apiResponse struct {
	Month []struct {
		Code int    `json:"code"`
		Name string `json:"name"`
		Date []struct {
			Year     int `json:"year"`
			Month    int `json:"month"`
			CurrBase struct {
				Value json.Number `json:"value"` // keeps the API's exact text
				Base  string      `json:"baseDesc"`
			} `json:"currBase"`
		} `json:"date"`
	} `json:"month"`
}

type point struct {
	Year, Month int
	Value       string
}

func snippet(b []byte) string {
	const max = 800
	if len(b) > max {
		return string(b[:max]) + "…(truncated)"
	}
	return string(b)
}

func getCPI(indexID, count int, lang string) ([]point, error) {
	q := url.Values{}
	q.Set("id", fmt.Sprintf("%d", indexID))
	q.Set("format", "json")
	q.Set("download", "false")
	q.Set("last", fmt.Sprintf("%d", count))
	q.Set("lang", lang)

	req, err := http.NewRequest(http.MethodGet, cbsAPI+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	//
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
			"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s: %s", resp.Status, snippet(body))
	}

	var data apiResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode failed: %w; body: %s", err, snippet(body))
	}
	if len(data.Month) == 0 {
		return nil, fmt.Errorf("no series returned; body: %s", snippet(body))
	}

	dates := data.Month[0].Date
	pts := make([]point, 0, len(dates))
	for _, d := range dates {
		base := d.CurrBase.Base
		switch {
		case base == "":
			// empty != rebase. Wrong field name or partial response — show the truth.
			return nil, fmt.Errorf("base came back empty; response shape differs from expected. Raw body: %s", snippet(body))
		case !strings.Contains(strings.ToLower(base), expectedBase):
			return nil, fmt.Errorf("%w: expected %q, got %q; review and re-sync before continuing",
				ErrBaseChanged, expectedBase, base)
		}
		pts = append(pts, point{Year: d.Year, Month: d.Month, Value: d.CurrBase.Value.String()})
	}

	sort.Slice(pts, func(i, j int) bool {
		if pts[i].Year != pts[j].Year {
			return pts[i].Year > pts[j].Year
		}
		return pts[i].Month > pts[j].Month
	})
	return pts, nil
}

func formatLine(p point) string {
	writeDate := time.Now().Format("02/01/06") // dd/mm/yy
	refDate := fmt.Sprintf("%02d/%02d/%02d", reportDay, p.Month, p.Year%100)
	return fmt.Sprintf("%s\t%s\t%s\t%s", writeDate, refDate, label, p.Value)
}

func main() {
	pts, err := getCPI(120010, 1, "en") // bump count for more rows
	if err != nil {
		if errors.Is(err, ErrBaseChanged) {
			fmt.Fprintf(os.Stderr, "[ALERT] %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}

	var b strings.Builder
	for _, p := range pts {
		b.WriteString(formatLine(p))
		b.WriteString("\r\n")
	}

	if err := os.WriteFile(outFile, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d line(s) to %s:\n%s", len(pts), outFile, b.String())
}

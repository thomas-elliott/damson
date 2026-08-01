package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

type assetRow struct {
	ID                 uint       `json:"id"`
	Name               string     `json:"name"`
	FileCount          int        `json:"file_count"`
	SizeBytes          int64      `json:"size_bytes"`
	NeedsReview        bool       `json:"needs_review"`
	CreatedAt          time.Time  `json:"created_at"`
	LicenceSPDX        string     `json:"licence_spdx"`
	LicenceStatus      string     `json:"licence_status"`
	PackageKind        string     `json:"package_kind"`
	Publisher          string     `json:"publisher"`
	ReleasedAt         *time.Time `json:"released_at"`
	UpdateAvailable    bool       `json:"update_available"`
	ThumbnailObjectKey string     `json:"thumbnail_object_key"`
}

type assetListResponse struct {
	Assets []assetRow `json:"assets"`
	Total  int64      `json:"total"`
}

type assetDetailResponse struct {
	ID                 uint       `json:"id"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	SourceURL          string     `json:"source_url"`
	NeedsReview        bool       `json:"needs_review"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Publisher          string     `json:"publisher"`
	PublisherURL       string     `json:"publisher_url"`
	Store              string     `json:"store"`
	StoreProductID     string     `json:"store_product_id"`
	ReleasedAt         *time.Time `json:"released_at"`
	StoreUpdatedAt     *time.Time `json:"store_updated_at"`
	LatestStoreVersion string     `json:"latest_store_version"`
	Engine             string     `json:"engine"`
	MinEngineVersion   string     `json:"min_engine_version"`
	Price              string     `json:"price"`
	PriceCurrency      string     `json:"price_currency"`
	Rating             *float64   `json:"rating"`
	RatingCount        *int       `json:"rating_count"`
	Licence            *struct {
		DetectedType string `json:"detected_type"`
		Status       string `json:"status"`
	} `json:"licence"`
	Tags []struct {
		ID   uint   `json:"id"`
		Path string `json:"path"`
	} `json:"tags"`
	Versions []struct {
		ID           uint   `json:"id"`
		VersionLabel string `json:"version_label"`
		StoreVersion string `json:"store_version"`
		PackageKind  string `json:"package_kind"`
		FileCount    int    `json:"file_count"`
		SizeBytes    int64  `json:"size_bytes"`
		IsCurrent    bool   `json:"is_current"`
	} `json:"versions"`
	Collections []struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"collections"`
}

// assetFilters are the query-param filters `dmsn assets` exposes. Every one
// maps onto a parameter GET /api/assets already accepts.
type assetFilters struct {
	page             pageFlags
	query            string
	tag              string
	collection       string
	project          string
	publisher        string
	store            string
	packageKind      string
	licenceStatus    string
	licenceSPDX      string
	sourceHost       string
	sort             string
	needsReview      bool
	updatesAvailable bool
	releasedAfter    string
}

func (f *assetFilters) register(fs *flag.FlagSet) {
	f.page.register(fs)
	fs.StringVar(&f.query, "q", "", "full-text search")
	fs.StringVar(&f.tag, "tag", "", "tag path, or a numeric tag id")
	fs.StringVar(&f.collection, "collection", "", "collection id")
	fs.StringVar(&f.project, "project", "", "project id")
	fs.StringVar(&f.publisher, "publisher", "", "exact publisher")
	fs.StringVar(&f.store, "store", "", "storefront (unity|sketchfab|itch)")
	fs.StringVar(&f.packageKind, "package-kind", "", "loose|zip|unitypackage")
	fs.StringVar(&f.licenceStatus, "licence-status", "", "ok|warn|err|unk")
	fs.StringVar(&f.licenceSPDX, "licence", "", "exact SPDX identifier")
	fs.StringVar(&f.sourceHost, "source-host", "", "host of the source URL")
	fs.StringVar(&f.sort, "sort", "", "created_desc|created_asc|released_desc|released_asc|name_asc")
	fs.BoolVar(&f.needsReview, "needs-review", false, "only assets awaiting review")
	fs.BoolVar(&f.updatesAvailable, "updates-available", false, "only assets with a newer store version")
	fs.StringVar(&f.releasedAfter, "released-after", "", "YYYY-MM-DD or RFC3339")
}

var assetSorts = []string{"created_desc", "created_asc", "released_desc", "released_asc", "name_asc"}

func (f *assetFilters) buildQuery() (url.Values, error) {
	q := url.Values{}
	if err := f.page.apply(q); err != nil {
		return nil, err
	}
	setIf(q, "q", f.query)
	// A tag is a path by default; digits mean the caller had an id in hand.
	if t := strings.TrimSpace(f.tag); t != "" {
		if _, err := strconv.ParseUint(t, 10, 64); err == nil {
			q.Set("tag_id", t)
		} else {
			q.Set("tag_path", t)
		}
	}
	for _, idFlag := range []struct{ key, value, name string }{
		{"collection_id", f.collection, "--collection"},
		{"project_id", f.project, "--project"},
	} {
		if v := strings.TrimSpace(idFlag.value); v != "" {
			if _, err := strconv.ParseUint(v, 10, 64); err != nil {
				return nil, axi.Usagef(
					[]string{binaryName + " collections", binaryName + " tags"},
					"%s must be a numeric id, got %q", idFlag.name, v)
			}
			q.Set(idFlag.key, v)
		}
	}
	setIf(q, "publisher", f.publisher)
	setIf(q, "store", f.store)
	setIf(q, "package_kind", f.packageKind)
	setIf(q, "licence_status", f.licenceStatus)
	setIf(q, "licence_spdx", f.licenceSPDX)
	setIf(q, "source_host", f.sourceHost)
	setIf(q, "released_after", f.releasedAfter)
	setBoolIf(q, "needs_review", f.needsReview)
	setBoolIf(q, "updates_available", f.updatesAvailable)

	// The server silently falls back to its default sort on an unknown value.
	// Reject it here instead: a caller who mistyped a sort should learn that
	// rather than quietly receive a differently-ordered list.
	if s := strings.TrimSpace(f.sort); s != "" {
		if !contains(assetSorts, s) {
			return nil, axi.Usagef(
				[]string{"valid sorts: " + strings.Join(assetSorts, ",")},
				"unknown sort %q", s)
		}
		q.Set("sort", s)
	}
	return q, nil
}

func init() {
	filters := &assetFilters{}
	register(&command{
		name:    "assets",
		summary: "list assets in the current library",
		usage: "usage: dmsn assets [filters]\n\n" +
			"filters: --q --tag --collection --project --publisher --store\n" +
			"         --package-kind --licence --licence-status --source-host\n" +
			"         --needs-review --updates-available --released-after --sort\n" +
			"         --limit --offset\n\n" +
			"Default fields: id,name,publisher,needs_review.\n" +
			"Widen with --fields; see the error from an unknown name for the full set.",
		flags: filters.register,
		run: func(ctx context.Context, e *Env) error {
			return runAssets(ctx, e, filters)
		},
	})
}

func runAssets(ctx context.Context, e *Env, f *assetFilters) error {
	client, err := e.Client()
	if err != nil {
		return err
	}
	q, err := f.buildQuery()
	if err != nil {
		return err
	}

	var resp assetListResponse
	if err := client.GetJSON(ctx, "/api/assets", q, &resp); err != nil {
		return err
	}

	rows := make([]map[string]any, 0, len(resp.Assets))
	for _, a := range resp.Assets {
		rows = append(rows, map[string]any{
			"id":               a.ID,
			"name":             a.Name,
			"publisher":        blankToNil(a.Publisher),
			"needs_review":     a.NeedsReview,
			"file_count":       a.FileCount,
			"size_bytes":       a.SizeBytes,
			"package_kind":     blankToNil(a.PackageKind),
			"licence_spdx":     blankToNil(a.LicenceSPDX),
			"licence_status":   blankToNil(a.LicenceStatus),
			"released_at":      a.ReleasedAt,
			"update_available": a.UpdateAvailable,
			"created_at":       a.CreatedAt,
		})
	}

	// The library-wide count is only needed to make an empty result
	// unambiguous, so it costs a round trip only when there is nothing to show.
	libraryTotal := int64(-1)
	if len(rows) == 0 {
		libraryTotal, _ = e.libraryAssetTotal(ctx, client)
	}
	table := axi.Table{
		Key:     "assets",
		Columns: []string{"id", "name", "publisher", "needs_review"},
		Allowed: []string{
			"id", "name", "publisher", "needs_review", "file_count", "size_bytes",
			"package_kind", "licence_spdx", "licence_status", "released_at",
			"update_available", "created_at",
		},
		Rows:      rows,
		Total:     int(resp.Total),
		EmptyNote: emptyNote("asset", describeFilters(q), libraryTotal),
	}

	help := []string{binaryName + " asset <id>"}
	help = append(help, f.page.pageHelp("assets", len(rows), resp.Total)...)
	if len(rows) > 0 {
		help = append(help, binaryName+" bulk tag --assets <ids> --tag <path>")
	}
	return axi.RenderTable(e.Out, table, help, e.Globals.options())
}

// libraryAssetTotal fetches the unfiltered asset count so an empty result can
// distinguish "nothing matches" from "the library is empty". Best-effort: a
// failure just omits the parenthetical.
func (e *Env) libraryAssetTotal(ctx context.Context, client *Client) (int64, error) {
	var ov overviewResponse
	if err := client.GetJSON(ctx, "/api/overview", nil, &ov); err != nil {
		return -1, err
	}
	return int64(ov.Counts.Assets), nil
}

func runAsset(ctx context.Context, e *Env) error {
	if len(e.Args) != 1 {
		return axi.Usagef(
			[]string{binaryName + " assets", binaryName + " asset <id>"},
			"asset takes exactly one id")
	}
	id, err := parsePositiveID(e.Args[0], "asset id")
	if err != nil {
		return err
	}

	client, err := e.Client()
	if err != nil {
		return err
	}
	var a assetDetailResponse
	if err := client.GetJSON(ctx, "/api/assets/"+id, nil, &a); err != nil {
		return err
	}

	tags := make([]any, 0, len(a.Tags))
	for _, t := range a.Tags {
		tags = append(tags, t.Path)
	}
	collections := make([]any, 0, len(a.Collections))
	for _, c := range a.Collections {
		collections = append(collections, c.Name)
	}
	versions := make([]any, 0, len(a.Versions))
	for _, v := range a.Versions {
		label := firstNonEmpty(v.StoreVersion, v.VersionLabel, "unversioned")
		if v.IsCurrent {
			label += "*"
		}
		versions = append(versions, label)
	}

	fields := []toon.KV{
		{Key: "id", Value: a.ID},
		{Key: "name", Value: a.Name},
		{Key: "publisher", Value: blankToNil(a.Publisher)},
		{Key: "store", Value: blankToNil(a.Store)},
		{Key: "needs_review", Value: a.NeedsReview},
		{Key: "description", Value: blankToNil(a.Description)},
		{Key: "source_url", Value: blankToNil(a.SourceURL)},
		{Key: "engine", Value: blankToNil(a.Engine)},
		{Key: "min_engine_version", Value: blankToNil(a.MinEngineVersion)},
		{Key: "latest_store_version", Value: blankToNil(a.LatestStoreVersion)},
		{Key: "released_at", Value: a.ReleasedAt},
		{Key: "store_updated_at", Value: a.StoreUpdatedAt},
		{Key: "price", Value: blankToNil(joinPrice(a.Price, a.PriceCurrency))},
		{Key: "rating", Value: derefFloat(a.Rating)},
		{Key: "rating_count", Value: derefInt(a.RatingCount)},
		{Key: "publisher_url", Value: blankToNil(a.PublisherURL)},
		{Key: "store_product_id", Value: blankToNil(a.StoreProductID)},
		{Key: "licence", Value: licenceValue(a)},
		{Key: "created_at", Value: a.CreatedAt},
		{Key: "updated_at", Value: a.UpdatedAt},
	}

	record := axi.Record{
		Key:         "asset",
		Fields:      fields,
		Truncatable: []string{"description"},
	}
	for _, f := range fields {
		record.Allowed = append(record.Allowed, f.Key)
	}

	// Versions, tags and collections are lists, so they ride alongside the
	// object block rather than being flattened into it.
	if err := axi.RenderRecord(e.Out, record, nil, e.Globals.options()); err != nil {
		return err
	}
	if !e.Globals.json {
		doc := &toon.Doc{}
		doc.List("versions", versions)
		doc.List("tags", tags)
		doc.List("collections", collections)
		fmt.Fprint(e.Out, doc.String())
	}

	return writeHelp(e, []string{
		fmt.Sprintf("%s asset edit %d --name <name>", binaryName, a.ID),
		fmt.Sprintf("%s asset tag %d <path>", binaryName, a.ID),
		fmt.Sprintf("%s asset review %d", binaryName, a.ID),
	})
}

func licenceValue(a assetDetailResponse) any {
	if a.Licence == nil || a.Licence.DetectedType == "" {
		return nil
	}
	return a.Licence.DetectedType
}

func joinPrice(price, currency string) string {
	if price == "" {
		return ""
	}
	if currency == "" {
		return price
	}
	return price + " " + currency
}

func derefFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func derefInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

// parsePositiveID validates a positional id so a typo produces a usage error
// rather than a 404 from the server.
func parsePositiveID(raw, what string) (string, error) {
	s := strings.TrimSpace(raw)
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n == 0 {
		return "", axi.Usagef(nil, "%s must be a positive number, got %q", what, raw)
	}
	return s, nil
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// unityRenderCall marks the inline script that boots the package page's React
// controller. Its single argument is the page's whole normalised entity store,
// which carries far more than the Coveo index exposes.
const unityRenderCall = "ReactDOMrender("

// unityListing is the subset of the listing page's entity store we map.
type unityListing struct {
	product   *unityProduct
	version   *unityProductVersion
	publisher *unityProductPublisher
	category  *unityCategory
	tags      []string
}

type unityEntityStore struct {
	Product          map[string]*unityProduct          `json:"Product"`
	ProductVersion   map[string]*unityProductVersion   `json:"ProductVersion"`
	ProductPublisher map[string]*unityProductPublisher `json:"ProductPublisher"`
	ProductTag       map[string]*unityProductTag       `json:"ProductTag"`
	Category         map[string]*unityCategory         `json:"Category"`
}

type unityRenderArg struct {
	Data struct {
		Entity unityEntityStore `json:"ENTITY"`
	} `json:"data"`
}

// unityEntityRef is the store's reference shape: {"type":"id","id":["Product","1"]}.
type unityEntityRef struct {
	ID []string `json:"id"`
}

func (r *unityEntityRef) key() string {
	if r == nil || len(r.ID) < 2 {
		return ""
	}
	return r.ID[1]
}

type unityProduct struct {
	ID                     string              `json:"id"`
	ProductID              string              `json:"productId"`
	Slug                   string              `json:"slug"`
	Name                   string              `json:"name"`
	Description            string              `json:"description"`
	AssetCount             int                 `json:"assetCount"`
	DownloadSize           string              `json:"downloadSize"`
	ReviewCount            int                 `json:"reviewCount"`
	PackageType            string              `json:"packageType"`
	PublishNotes           string              `json:"publishNotes"`
	LicenseText            string              `json:"licenseText"`
	CustomLicense          bool                `json:"customLicense"`
	FirstPublishedDate     string              `json:"firstPublishedDate"`
	SupportedUnityVersions []string            `json:"supportedUnityVersions"`
	Images                 []unityProductImage `json:"images"`
	Rating                 *unityRating        `json:"rating"`
	Price                  *unityPrice         `json:"originalPrice"`
	SRPs                   []unitySRP          `json:"srps"`
	CurrentVersion         *unityEntityRef     `json:"currentVersion"`
	Publisher              *unityEntityRef     `json:"publisher"`
	Category               *unityEntityRef     `json:"category"`
	PopularTags            []unityEntityRef    `json:"popularTags"`
}

type unityRating struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

type unityPrice struct {
	FinalPrice      string `json:"finalPrice"`
	Currency        string `json:"currency"`
	IsFree          bool   `json:"isFree"`
	EntitlementType string `json:"entitlementType"`
}

type unitySRP struct {
	Version string   `json:"version"`
	Types   []string `json:"types"`
}

type unityProductVersion struct {
	Name          string `json:"name"`
	PublishedDate string `json:"publishedDate"`
}

type unityProductPublisher struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	SupportURL string `json:"supportUrl"`
}

type unityProductTag struct {
	Name string `json:"name"`
}

type unityCategory struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// unityFetchListing reads the package page and parses its inlined entity store.
// Best-effort by design: any transport, status or parse failure yields nil so
// the caller keeps the Coveo-only metadata rather than failing the fetch.
func unityFetchListing(ctx context.Context, client *Client, listingURL, productID string) (*unityListing, string) {
	if listingURL == "" {
		return nil, ""
	}
	resp, err := client.Get(ctx, listingURL, nil)
	if err != nil || resp.StatusCode >= 400 {
		return nil, ""
	}
	pageURL := firstNonEmpty(resp.FinalURL, listingURL)
	l, err := unityParseListing(resp.Body, productID)
	if err != nil {
		return nil, pageURL
	}
	return l, pageURL
}

// unityParseListing extracts the entity store from a package page body and
// resolves the product's referenced version, publisher, category and tags.
func unityParseListing(body []byte, productID string) (*unityListing, error) {
	arg, err := unityRenderArgument(body)
	if err != nil {
		return nil, err
	}
	var parsed unityRenderArg
	if err := json.Unmarshal(arg, &parsed); err != nil {
		return nil, fmt.Errorf("unity: decode listing payload: %w", err)
	}
	store := parsed.Data.Entity
	product := unityPickProduct(store.Product, productID)
	if product == nil {
		return nil, fmt.Errorf("unity: no product in listing payload")
	}
	l := &unityListing{product: product}
	if v := store.ProductVersion[product.CurrentVersion.key()]; v != nil {
		l.version = v
	}
	if p := store.ProductPublisher[product.Publisher.key()]; p != nil {
		l.publisher = p
	}
	if c := store.Category[product.Category.key()]; c != nil {
		l.category = c
	}
	for i := range product.PopularTags {
		if t := store.ProductTag[product.PopularTags[i].key()]; t != nil {
			if name := strings.TrimSpace(t.Name); name != "" {
				l.tags = append(l.tags, name)
			}
		}
	}
	return l, nil
}

// unityPickProduct resolves the page's product: by id when it is present in the
// store, else the sole entry (a package page carries exactly one).
func unityPickProduct(products map[string]*unityProduct, productID string) *unityProduct {
	if p := products[productID]; p != nil {
		return p
	}
	if len(products) == 1 {
		for _, p := range products {
			return p
		}
	}
	return nil
}

// unityRenderArgument returns the raw JSON object passed to ReactDOMrender, by
// scanning for the matching close brace. A plain json.Decoder can't be used
// directly: the call sits mid-script, so the argument is followed by trailing
// JavaScript rather than EOF.
func unityRenderArgument(body []byte) ([]byte, error) {
	s := string(body)
	call := strings.Index(s, unityRenderCall)
	if call < 0 {
		return nil, fmt.Errorf("unity: no %s in listing page", unityRenderCall)
	}
	open := strings.Index(s[call:], "{")
	if open < 0 {
		return nil, fmt.Errorf("unity: no argument object in listing page")
	}
	start := call + open
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		switch c := s[i]; {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return []byte(s[start : i+1]), nil
			}
		}
	}
	return nil, fmt.Errorf("unity: unterminated argument object in listing page")
}

// unityApplyListing merges listing-payload data into Coveo-derived metadata.
// Fields Coveo already supplied are left alone; everything Coveo cannot express
// (dates, store version, engine compatibility, the spec sheet) is set here.
func unityApplyListing(md *Metadata, l *unityListing, pageURL string) {
	if md == nil || l == nil || l.product == nil {
		return
	}
	p := l.product

	if md.Title == "" {
		md.Title = p.Name
	}
	if md.Description == "" {
		md.Description = htmlToMarkdown(p.Description)
	}
	if md.StoreID == "" {
		md.StoreID = p.ID
	}
	if len(md.ImageURLs) == 0 {
		md.ImageURLs = unityGalleryURLs(p.Images, pageURL)
	}
	if md.CoverURL == "" && len(md.ImageURLs) > 0 {
		md.CoverURL = md.ImageURLs[0]
	}

	md.Engine = "unity"
	if md.ReleasedAt == nil {
		md.ReleasedAt = unityParseTime(p.FirstPublishedDate)
	}
	if md.MinEngineVersion == "" && len(p.SupportedUnityVersions) > 0 {
		md.MinEngineVersion = unityShortVersion(p.SupportedUnityVersions[0])
	}
	if l.version != nil {
		md.LatestStoreVersion = strings.TrimSpace(l.version.Name)
		if md.StoreUpdatedAt == nil {
			md.StoreUpdatedAt = unityParseTime(l.version.PublishedDate)
		}
	}
	if l.publisher != nil {
		if md.Publisher == "" {
			md.Publisher = strings.TrimSpace(l.publisher.Name)
		}
		md.PublisherURL = firstNonEmpty(
			strings.TrimSpace(l.publisher.URL),
			strings.TrimSpace(l.publisher.SupportURL),
			md.PublisherURL,
		)
	}
	if l.category != nil && md.CategoryPath == "" {
		md.CategoryPath = strings.TrimSpace(l.category.Slug)
	}
	if len(l.tags) > 0 {
		md.Tags = dedupeTags(append(md.Tags, l.tags...))
	}
	// Coveo's ec_rating carries more precision than the payload's rounded
	// average, so it wins when both answered.
	if md.Rating == 0 && p.Rating != nil {
		md.Rating = p.Rating.Average
	}
	if md.RatingCount == 0 {
		if p.Rating != nil && p.Rating.Count > 0 {
			md.RatingCount = p.Rating.Count
		} else if p.ReviewCount > 0 {
			md.RatingCount = p.ReviewCount
		}
	}
	if p.Price != nil {
		if md.Price == "" {
			md.Price = strings.TrimSpace(p.Price.FinalPrice)
		}
		if c := strings.TrimSpace(p.Price.Currency); c != "" {
			md.Currency = c
		}
	}

	unityListingAttributes(md, l)
	unityListingLicence(md, p)
}

// unityListingAttributes records the store-specific long tail, in the order the
// spec sheet should render it.
func unityListingAttributes(md *Metadata, l *unityListing) {
	p := l.product
	if len(p.SupportedUnityVersions) > 0 {
		md.AddAttribute("supported_unity_versions", "Supported Unity versions",
			strings.Join(p.SupportedUnityVersions, ", "))
	}
	md.AddAttribute("download_size", "Download size", unityHumanSize(p.DownloadSize))
	if p.AssetCount > 0 {
		md.AddAttribute("asset_count", "Assets", strconv.Itoa(p.AssetCount))
	}
	md.AddAttribute("package_type", "Package type", p.PackageType)
	if len(p.SRPs) > 0 {
		var pipelines []string
		for _, srp := range p.SRPs {
			pipelines = append(pipelines, srp.Types...)
		}
		md.AddAttribute("render_pipelines", "Render pipelines", strings.Join(dedupeTags(pipelines), ", "))
	}
	if p.Price != nil {
		if p.Price.IsFree {
			md.AddAttribute("entitlement", "Entitlement", "Free")
		} else {
			md.AddAttribute("entitlement", "Entitlement", unityEntitlementLabel(p.Price.EntitlementType))
		}
	}
	if notes := htmlToMarkdown(p.PublishNotes); notes != "" {
		md.AddAttribute("changelog", "Release notes", notes)
	}
}

// unityListingLicence upgrades the blanket EULA hint Coveo forces with the
// per-asset licence detail only the listing payload carries.
func unityListingLicence(md *Metadata, p *unityProduct) {
	text := strings.TrimSpace(stripTags(p.LicenseText))
	if p.CustomLicense && text != "" {
		md.LicenceText = text
		md.LicenceHint = ""
		return
	}
	if p.Price != nil && strings.EqualFold(p.Price.EntitlementType, "SERVICE") {
		md.LicenceHint = "unity-eula"
	}
}

func unityEntitlementLabel(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "":
		return ""
	case "SERVICE":
		return "Standard Unity Asset Store EULA"
	case "SUBSCRIPTION":
		return "Included with subscription"
	default:
		return t
	}
}

// unityParseTime reads the payload's ISO-8601 timestamps. Returns nil for a
// blank or unparseable value so an absent date stays absent.
func unityParseTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}

// unityShortVersion trims a Unity build version to its release form:
// "2020.3.0f1" and "2020.3.0" both become "2020.3".
func unityShortVersion(s string) string {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return s
	}
	return parts[0] + "." + parts[1]
}

// unityHumanSize formats the payload's byte-count string for display. A
// non-numeric value is passed through unchanged.
func unityHumanSize(raw string) string {
	raw = strings.TrimSpace(raw)
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 {
		return strconv.FormatInt(int64(n), 10) + " " + units[i]
	}
	return strconv.FormatFloat(n, 'f', 1, 64) + " " + units[i]
}

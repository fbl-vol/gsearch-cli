package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	baseURL = "https://api.dataforsyningen.dk/rest/gsearch/v2.0"
	version = "0.1.0"
)

var supportedResources = []string{
	"adresse",
	"husnummer",
	"navngivenvej",
	"stednavn",
	"kommune",
	"region",
	"retskreds",
	"postnummer",
	"opstillingskreds",
	"sogn",
	"politikreds",
	"matrikel",
	"matrikel_udgaaet",
}

type result map[string]any

type searchOptions struct {
	Token   string
	Limit   int
	SRID    int
	Filter  string
	Timeout time.Duration
}

type addressSelection struct {
	Provider    string   `json:"provider"`
	Resource    string   `json:"resource"`
	ID          string   `json:"id,omitempty"`
	HusnummerID string   `json:"husnummerId,omitempty"`
	Label       string   `json:"label,omitempty"`
	Kommunekode string   `json:"kommunekode,omitempty"`
	Vejkode     string   `json:"vejkode,omitempty"`
	Postnummer  string   `json:"postnummer,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Raw         result   `json:"raw"`
}

type nearestResponse struct {
	Postnummernavn string   `json:"postnummernavn"`
	RadiusMeters   int      `json:"radius_meters"`
	Candidates     []result `json:"candidates"`
}

var (
	httpClient        = &http.Client{Transport: gsearchTransport()}
	tokenParamPattern = regexp.MustCompile(`([?&](?:token|TOKEN)=)[^&\s]+`)
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version":
		fmt.Println(version)
		return nil
	case "resources":
		return runResources(args[1:])
	case "search":
		return runSearch(args[1:])
	case "address":
		return runAddress(args[1:])
	case "spatial":
		return runSpatial(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\nRun: gsearch-cli --help", args[0])
	}
}

func runResources(args []string) error {
	fs := newFlagSet("resources")
	compact := fs.Bool("compact", false, "emit compact JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return writeJSON(supportedResources, *compact)
}

func runSearch(args []string) error {
	fs := newFlagSet("search")
	options := addSearchFlags(fs, 4326)
	compact := fs.Bool("compact", false, "emit compact JSON")
	rest, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 2 {
		return errors.New("usage: gsearch-cli search <resource> <query> [--limit 10] [--filter ECQL]")
	}
	resource := rest[0]
	if !isSupportedResource(resource) {
		return fmt.Errorf("unsupported resource %q; run: gsearch-cli resources", resource)
	}
	query := strings.Join(rest[1:], " ")

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	results, err := searchGSearch(ctx, resource, query, options)
	if err != nil {
		return err
	}
	return writeJSON(results, *compact)
}

func runAddress(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("Usage: gsearch-cli address suggest <query> [--limit 10] [--compact]")
		return nil
	}
	switch args[0] {
	case "suggest":
		return runAddressSuggest(args[1:])
	default:
		return fmt.Errorf("unknown address command %q", args[0])
	}
}

func runAddressSuggest(args []string) error {
	fs := newFlagSet("address suggest")
	options := addSearchFlags(fs, 4326)
	compact := fs.Bool("compact", false, "emit compact JSON")
	rest, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	query := strings.Join(rest, " ")
	if query == "" {
		return errors.New("usage: gsearch-cli address suggest <query>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	suggestions, err := searchAddressSuggestions(ctx, query, options)
	if err != nil {
		return err
	}
	return writeJSON(suggestions, *compact)
}

func runSpatial(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("Usage: gsearch-cli spatial nearest-husnummer --easting 689255 --northing 6051787")
		return nil
	}
	switch args[0] {
	case "nearest-husnummer":
		return runNearestHusnummer(args[1:])
	default:
		return fmt.Errorf("unknown spatial command %q", args[0])
	}
}

func runNearestHusnummer(args []string) error {
	fs := newFlagSet("spatial nearest-husnummer")
	options := addSearchFlags(fs, 25832)
	easting := fs.Int("easting", 689255, "EPSG:25832 easting")
	northing := fs.Int("northing", 6051787, "EPSG:25832 northing")
	postalPrefix := fs.String("postal-prefix", "4", "postal prefix used to seed postnummer lookup")
	radiiRaw := fs.String("radii", "100,300,1000", "comma-separated radii in meters")
	compact := fs.Bool("compact", false, "emit compact JSON")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	radii, err := parseRadii(*radiiRaw)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	response, err := nearestHusnummer(ctx, *easting, *northing, *postalPrefix, radii, options)
	if err != nil {
		return err
	}
	return writeJSON(response, *compact)
}

func runDoctor(args []string) error {
	fs := newFlagSet("doctor")
	options := addSearchFlags(fs, 4326)
	compact := fs.Bool("compact", false, "emit compact JSON")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	results, err := searchGSearch(ctx, "husnummer", "genvej", options)
	status := map[string]any{
		"service":  "dataforsyningen-gsearch",
		"base_url": baseURL,
		"resource": "husnummer",
		"query":    "genvej",
		"ok":       err == nil,
	}
	if err != nil {
		status["error"] = err.Error()
	} else {
		status["count"] = len(results)
		if len(results) > 0 {
			status["first"] = stringField(results[0], "visningstekst")
		}
	}
	return writeJSON(status, *compact)
}

func addSearchFlags(fs *flag.FlagSet, defaultSRID int) *searchOptions {
	options := &searchOptions{}
	fs.StringVar(&options.Token, "token", "", "Dataforsyningen token; defaults to GSEARCH_TOKEN")
	fs.IntVar(&options.Limit, "limit", 10, "maximum results")
	fs.IntVar(&options.SRID, "srid", defaultSRID, "response geometry SRID")
	fs.StringVar(&options.Filter, "filter", "", "ECQL filter expression")
	fs.DurationVar(&options.Timeout, "timeout", 10*time.Second, "request timeout")
	return options
}

func searchAddressSuggestions(ctx context.Context, query string, options *searchOptions) ([]addressSelection, error) {
	var wg sync.WaitGroup
	var husnummer []result
	var adresse []result
	var husnummerErr error
	var adresseErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		husnummer, husnummerErr = searchGSearch(ctx, "husnummer", query, options)
	}()
	go func() {
		defer wg.Done()
		adresse, adresseErr = searchGSearch(ctx, "adresse", query, options)
	}()
	wg.Wait()

	if husnummerErr != nil && adresseErr != nil {
		return nil, fmt.Errorf("%v; %v", husnummerErr, adresseErr)
	}
	if husnummerErr != nil {
		return nil, husnummerErr
	}
	if adresseErr != nil {
		return nil, adresseErr
	}
	return mergeAddressResults(husnummer, adresse), nil
}

func mergeAddressResults(husnummer []result, adresse []result) []addressSelection {
	husnummerIDs := map[string]string{}
	for _, item := range husnummer {
		key := addressKey(item, "husnummertekst")
		if key != "" {
			husnummerIDs[key] = stringField(item, "id")
		}
	}

	seen := map[string]bool{}
	merged := []addressSelection{}

	for _, item := range adresse {
		key := addressKey(item, "husnummer")
		normalized := normalizeAddressResult("adresse", item, husnummerIDs[key])
		if normalized.Label != "" && !seen[normalized.Label] {
			seen[normalized.Label] = true
			merged = append(merged, normalized)
		}
	}

	for _, item := range husnummer {
		id := stringField(item, "id")
		normalized := normalizeAddressResult("husnummer", item, id)
		if normalized.Label != "" && !seen[normalized.Label] {
			seen[normalized.Label] = true
			merged = append(merged, normalized)
		}
	}

	return merged
}

func nearestHusnummer(ctx context.Context, easting int, northing int, postalPrefix string, radii []int, options *searchOptions) (nearestResponse, error) {
	point := fmt.Sprintf("POINT(%d %d)", easting, northing)

	postnummerOptions := *options
	postnummerOptions.Limit = 5
	postnummerOptions.Filter = fmt.Sprintf("INTERSECTS(geometri,%s)", point)
	postnumre, err := searchGSearch(ctx, "postnummer", postalPrefix, &postnummerOptions)
	if err != nil {
		return nearestResponse{}, err
	}
	if len(postnumre) == 0 {
		return nearestResponse{}, fmt.Errorf("no containing postnummer found for %s", point)
	}
	postnummernavn := stringField(postnumre[0], "postnummernavn")
	if postnummernavn == "" {
		postnummernavn = stringField(postnumre[0], "visningstekst")
	}
	if postnummernavn == "" {
		return nearestResponse{}, fmt.Errorf("postnummer lookup returned no postnummernavn for %s", point)
	}

	for _, radius := range radii {
		husnummerOptions := *options
		husnummerOptions.Limit = 5
		husnummerOptions.Filter = fmt.Sprintf("DWITHIN(geometri,%s,%d,meters)", point, radius)
		candidates, err := searchGSearch(ctx, "husnummer", postnummernavn, &husnummerOptions)
		if err != nil {
			return nearestResponse{}, err
		}
		if len(candidates) > 0 {
			return nearestResponse{
				Postnummernavn: postnummernavn,
				RadiusMeters:   radius,
				Candidates:     candidates,
			}, nil
		}
	}

	return nearestResponse{}, fmt.Errorf("no nearby husnummer found within %d meters of %s", radii[len(radii)-1], point)
}

func searchGSearch(ctx context.Context, resource string, query string, options *searchOptions) ([]result, error) {
	token := options.Token
	if token == "" {
		token = os.Getenv("GSEARCH_TOKEN")
	}
	if token == "" {
		return nil, errors.New("GSEARCH_TOKEN is required")
	}
	if options.Limit <= 0 {
		return nil, errors.New("limit must be positive")
	}

	endpoint, err := url.Parse(baseURL + "/" + resource)
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("token", token)
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(options.Limit))
	params.Set("srid", strconv.Itoa(options.SRID))
	if options.Filter != "" {
		params.Set("filter", options.Filter)
	}
	endpoint.RawQuery = params.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("accept", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GSearch %s request failed: %s", resource, redactToken(err.Error()))
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("GSearch %s failed: HTTP %d %s", resource, response.StatusCode, string(body))
	}

	var payload []result
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func normalizeAddressResult(resource string, item result, husnummerID string) addressSelection {
	longitude, latitude := firstLonLat(item["geometri"])
	return addressSelection{
		Provider:    "dataforsyningen-gsearch",
		Resource:    resource,
		ID:          stringField(item, "id"),
		HusnummerID: husnummerID,
		Label:       stringField(item, "visningstekst"),
		Kommunekode: stringField(item, "kommunekode"),
		Vejkode:     stringField(item, "vejkode"),
		Postnummer:  stringField(item, "postnummer"),
		Longitude:   longitude,
		Latitude:    latitude,
		Raw:         item,
	}
}

func firstLonLat(geometry any) (*float64, *float64) {
	geometryMap, ok := geometry.(map[string]any)
	if !ok {
		return nil, nil
	}
	position, ok := firstPosition(geometryMap["coordinates"])
	if !ok {
		return nil, nil
	}
	return &position[0], &position[1]
}

func firstPosition(value any) ([2]float64, bool) {
	values, ok := value.([]any)
	if !ok {
		return [2]float64{}, false
	}
	if len(values) >= 2 {
		x, xOK := values[0].(float64)
		y, yOK := values[1].(float64)
		if xOK && yOK {
			return [2]float64{x, y}, true
		}
	}
	for _, child := range values {
		position, ok := firstPosition(child)
		if ok {
			return position, true
		}
	}
	return [2]float64{}, false
}

func addressKey(item result, houseNumberField string) string {
	parts := []string{
		stringField(item, "kommunekode"),
		stringField(item, "vejkode"),
		stringField(item, houseNumberField),
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, ":")
}

func stringField(item result, key string) string {
	value, ok := item[key].(string)
	if !ok {
		return ""
	}
	return value
}

func parseRadii(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	radii := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		radius, err := strconv.Atoi(part)
		if err != nil || radius <= 0 {
			return nil, fmt.Errorf("invalid radius %q", part)
		}
		radii = append(radii, radius)
	}
	if len(radii) == 0 {
		return nil, errors.New("at least one radius is required")
	}
	return radii, nil
}

func isSupportedResource(resource string) bool {
	for _, candidate := range supportedResources {
		if resource == candidate {
			return true
		}
	}
	return false
}

func writeJSON(value any, compact bool) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if !compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	flagArgs := []string{}
	positionals := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}

		flagArgs = append(flagArgs, arg)
		name := strings.TrimLeft(arg, "-")
		if before, _, ok := strings.Cut(name, "="); ok {
			name = before
		}
		defined := fs.Lookup(name)
		if defined == nil || strings.Contains(arg, "=") || isBoolFlag(defined) {
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag needs an argument: %s", arg)
		}
		i++
		flagArgs = append(flagArgs, args[i])
	}

	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}
	return positionals, nil
}

func isBoolFlag(f *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	value, ok := f.Value.(boolFlag)
	return ok && value.IsBoolFlag()
}

func gsearchTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The Dataforsyningen gateway can close Go HTTP/2 requests with unexpected EOF.
	transport.ForceAttemptHTTP2 = false
	transport.TLSClientConfig = &tls.Config{NextProtos: []string{"http/1.1"}}
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	return transport
}

func redactToken(message string) string {
	return tokenParamPattern.ReplaceAllString(message, "${1}<redacted>")
}

func printUsage() {
	fmt.Println(`gsearch-cli - SDFI/Dataforsyningen GSearch from the command line

Usage:
  gsearch-cli resources [--compact]
  gsearch-cli search <resource> <query> [--limit 10] [--srid 4326] [--filter ECQL]
  gsearch-cli address suggest <query> [--limit 10]
  gsearch-cli spatial nearest-husnummer --easting 689255 --northing 6051787
  gsearch-cli doctor
  gsearch-cli version

Environment:
  GSEARCH_TOKEN  Dataforsyningen token

GSearch is SDFI/Dataforsyningen GSearch, not Google Search.`)
}

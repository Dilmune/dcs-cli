package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dilmune/dcs-cli/internal/client"
)

// minIDPrefixLength is the length of the short IDs the list commands print, so
// a prefix copied from a table resolves and a one or two letter guess does not.
const minIDPrefixLength = shortIDLength

const (
	pageQuery    = "page"
	perPageQuery = "perPage"
	// lookupPageSize is the largest page the API serves. A lookup reads every
	// page, because a name that is unique on the first page can repeat on the
	// next, and a resource past the first page must still be found.
	lookupPageSize     = 100
	maxCandidatesShown = 5
)

var (
	errLookupInputRequired = errors.New("lookup input is required")
	errLookupNotFound      = errors.New("no resource matches the lookup")
	errLookupAmbiguous     = errors.New("lookup matches more than one resource")
)

// lookup describes one kind of resource a command finds from what the user
// typed: a name (or domain) first, then a full ID, then a unique ID prefix.
// A lookup without a name function, for deployments and backups, starts at
// the full ID.
type lookup[T any] struct {
	kind     string
	wanted   string
	listHint string
	name     func(T) string
	id       func(T) string
}

// find refuses blank input before any request, so an unset shell variable can
// never reach the list call, then picks the one resource the input names.
func (l lookup[T]) find(ctx context.Context, input string, list func(context.Context) ([]T, error)) (*T, error) {
	if isBlank(input) {
		return nil, l.inputRequired()
	}
	items, err := list(ctx)
	if err != nil {
		return nil, fmt.Errorf("list %ss: %w", l.kind, err)
	}
	return l.pick(input, items)
}

func (l lookup[T]) pick(input string, items []T) (*T, error) {
	if isBlank(input) {
		return nil, l.inputRequired()
	}
	if l.hasNames() {
		if found, err := l.single(input, items, func(item T) bool {
			return strings.EqualFold(l.name(item), input)
		}); found != nil || err != nil {
			return found, err
		}
	}
	for i := range items {
		if l.id(items[i]) == input {
			return &items[i], nil
		}
	}
	if isLongEnoughForIDPrefix(input) {
		if found, err := l.single(input, items, func(item T) bool {
			return strings.HasPrefix(l.id(item), input)
		}); found != nil || err != nil {
			return found, err
		}
	}
	return nil, l.notFound(input, items)
}

// single returns the one item that matches, nil when none does, and an
// ambiguity error when several do. Picking the first of several is how a
// delete lands on the wrong resource.
func (l lookup[T]) single(input string, items []T, matches func(T) bool) (*T, error) {
	var found []*T
	for i := range items {
		if matches(items[i]) {
			found = append(found, &items[i])
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	default:
		return nil, l.ambiguous(input, found)
	}
}

func (l lookup[T]) inputRequired() error {
	return &client.CLIError{
		Message:    sentenceCase(l.wanted) + " is required.",
		Suggestion: l.listHint,
		Err:        errLookupInputRequired,
	}
}

func (l lookup[T]) notFound(input string, items []T) error {
	suggestion := l.listHint
	if isShortIDPrefix(input, items, l.id) {
		suggestion = fmt.Sprintf("An ID prefix needs at least %d characters. %s", minIDPrefixLength, l.listHint)
	}
	return &client.CLIError{
		Message:    fmt.Sprintf("%s %q not found.", sentenceCase(l.kind), input),
		Suggestion: suggestion,
		Err:        errLookupNotFound,
	}
}

func (l lookup[T]) ambiguous(input string, found []*T) error {
	shown := found[:min(len(found), maxCandidatesShown)]
	candidates := make([]string, 0, len(shown)+1)
	for _, item := range shown {
		candidates = append(candidates, l.describe(*item))
	}
	if hidden := len(found) - len(shown); hidden > 0 {
		candidates = append(candidates, fmt.Sprintf("%d more", hidden))
	}
	return &client.CLIError{
		Message:    fmt.Sprintf("%s %q is ambiguous: it matches %s.", sentenceCase(l.kind), input, joinAnd(candidates)),
		Suggestion: "Use the full ID to choose one.",
		Err:        errLookupAmbiguous,
	}
}

func (l lookup[T]) hasNames() bool {
	return l.name != nil
}

func (l lookup[T]) describe(item T) string {
	if !l.hasNames() {
		return l.id(item)
	}
	return fmt.Sprintf("%s (%s)", l.name(item), l.id(item))
}

func isLongEnoughForIDPrefix(input string) bool {
	return utf8.RuneCountInString(input) >= minIDPrefixLength
}

func isShortIDPrefix[T any](input string, items []T, id func(T) string) bool {
	if isLongEnoughForIDPrefix(input) {
		return false
	}
	for _, item := range items {
		if strings.HasPrefix(id(item), input) {
			return true
		}
	}
	return false
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func sentenceCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func joinAnd(parts []string) string {
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// listAll reads every page of a list endpoint whose data is a JSON array.
func listAll[T any](ctx context.Context, path string) ([]T, error) {
	return listPages(ctx, path, perPageQuery, client.Decode[[]T])
}

// listPages follows the page count the API reports, not the number of items a
// page decodes to: the storage listing drops nested files after paging, so one
// of its pages can be empty while a later one is not. A response without
// pagination metadata is read once.
func listPages[T any](ctx context.Context, path, perPageKey string, decode func(*client.APIResponse) ([]T, error)) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		query := url.Values{pageQuery: {strconv.Itoa(page)}, perPageKey: {strconv.Itoa(lookupPageSize)}}
		resp, err := apiClient.Get(ctx, path, query)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		items, err := decode(resp)
		if err != nil {
			return nil, fmt.Errorf("decode page %d: %w", page, err)
		}
		all = append(all, items...)
		if resp.Meta == nil || page >= resp.Meta.TotalPages {
			return all, nil
		}
	}
}

// requiredValue is a string flag that refuses a blank value. A script that
// runs `--server "$SERVER"` with the variable unset then fails, where a plain
// string flag would read as "not given" and fall back to a default.
type requiredValue struct {
	target *string
	wanted string
}

func (v *requiredValue) String() string {
	return *v.target
}

func (v *requiredValue) Set(value string) error {
	if isBlank(value) {
		return &client.CLIError{Message: v.wanted + " must not be empty", Err: errLookupInputRequired}
	}
	*v.target = value
	return nil
}

// Type keeps the help text identical to a plain string flag.
func (v *requiredValue) Type() string {
	return "string"
}

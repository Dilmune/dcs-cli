package workspace

import (
	"context"
	"io"

	"github.com/dilmune/dcs-cli/internal/ui"
)

type Kind uint8

const (
	Servers Kind = iota + 1
	SiteServers
	Sites
	ServerDetail
	SiteDetail
)

type Request struct {
	Kind     Kind
	ServerID string
	SiteID   string
	Page     int
}

type Field struct{ Label, Value string }

// Labels whose value is a status word. The producer uses these so the
// workspace can render the glyph without guessing from the value.
const (
	LabelStatus = "Status"
	LabelSSL    = "SSL"
)

// IsStatus reports whether the field renders as glyph plus word.
func (f Field) IsStatus() bool { return f.Label == LabelStatus || f.Label == LabelSSL }

type logoRow struct{ upper, lower string }

type logoSize struct{ width, height int }

type welcomeLayout struct {
	logo        logoSize
	gap         int
	headerAbove bool
}

// Status is the resource's state word; the subtitle leads with its glyph and
// Description carries the rest ("hetzner · hel1").
type Item struct {
	ID, Title, Description, Status, Command, Body string
	Fields                                        []Field
	Children                                      []Item
	Request                                       *Request
}

func (i Item) CanOpen() bool {
	return i.Request != nil || len(i.Children) > 0 || i.Command != "" || i.Body != "" || len(i.Fields) > 0
}

type Source interface {
	Identify(context.Context) (string, error)
	Load(context.Context, Request) (Item, error)
}

type Options struct {
	Input           io.Reader
	Output          io.Writer
	Catalog         Item
	Source          Source
	ShowWelcome     bool
	RememberWelcome func() error
	NoColor         bool
	Mode            ui.Mode
	Version         string
}

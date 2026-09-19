package workspace

import (
	"context"
	"io"
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

type logoRow struct{ upper, lower string }

type logoSize struct{ width, height int }

type welcomeLayout struct {
	logo        logoSize
	gap         int
	headerAbove bool
}

type Item struct {
	ID, Title, Description, Command, Body string
	Fields                                []Field
	Children                              []Item
	Request                               *Request
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
	Theme           string
	Version         string
}

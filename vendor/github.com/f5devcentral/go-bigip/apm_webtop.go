package bigip

import (
	"context"
	"encoding/json"
)

type (
	WebtopType        string
	CustomizationType string
	InitialState      string
	LinkType          string
)

const (
	WebtopTypePortal          WebtopType        = "portal-access"
	WebtopTypeFull            WebtopType        = "full"
	WebtopTypeNetwork         WebtopType        = "network-access"
	CustomizationTypeModern   CustomizationType = "Modern"
	CustomizationTypeStandard CustomizationType = "Standard"
	InitialStateCollapsed     InitialState      = "Collapsed"
	InitialStateExpanded      InitialState      = "Expanded"
	LinkTypeUri               LinkType          = "uri"
)

type BooledString bool

// Some endpoints have a "booledString" a boolean value that is represented as a string in the json payload
func (b BooledString) MarshalJSON() ([]byte, error) {
	str := "false"
	if b {
		str = "true"
	}
	return json.Marshal(str)
}

func (b *BooledString) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*b = str == "true"
	return nil
}

type Webtop struct {
	TMPartition string `json:"tmPartition,omitempty"`
	Partition   string `json:"partition,omitempty"`
	Name        string `json:"name"`
	WebtopConfig
}
type WebtopConfig struct {
	Description        string            `json:"description,omitempty"`
	CustomizationGroup string            `json:"customizationGroup"`
	InitialState       InitialState      `json:"initialState,omitempty"`
	CustomizationType  CustomizationType `json:"customizationType,omitempty"`
	LinkType           LinkType          `json:"linkType,omitempty"`
	Type               WebtopType        `json:"webtopType,omitempty"`
	ShowSearch         BooledString      `json:"showSearch"`
	WarningOnClose     BooledString      `json:"warningOnClose"`
	UrlEntryField      BooledString      `json:"urlEntryField"`
	ResourceSearch     BooledString      `json:"resourceSearch"`
	MinimizeToTray     BooledString      `json:"minimizeToTray"`
	LocationSpecific   BooledString      `json:"locationSpecific"`
}

type WebtopRead struct {
	FullPath                    string `json:"fullPath,omitempty"`
	SelfLink                    string `json:"selfLink,omitempty"`
	CustomizationGroupReference struct {
		Link string `json:"link,omitempty"`
	} `json:"customizationGroupReference,omitempty"`
	Webtop
	Generation int `json:"generation,omitempty"`
}

func (b *BigIP) CreateWebtop(ctx context.Context, webtop Webtop) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return b.post(webtop, uriMgmt, uriTm, uriApm, uriResource, uriWebtop)
}

func (b *BigIP) DeleteWebtop(ctx context.Context, name string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return b.delete(uriMgmt, uriTm, uriApm, uriResource, uriWebtop, name)
}

func (b *BigIP) GetWebtop(ctx context.Context, name string) (*WebtopRead, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var webtop WebtopRead
	err, _ := b.getForEntity(&webtop, uriMgmt, uriTm, uriApm, uriResource, uriWebtop, name)
	return &webtop, err
}

func (b *BigIP) ModifyWebtop(ctx context.Context, name string, webtop WebtopConfig) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return b.patch(webtop, uriMgmt, uriTm, uriApm, uriResource, uriWebtop, name)
}

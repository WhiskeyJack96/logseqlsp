package logseq

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/WhiskeyJack96/logseqlsp/files"
	"github.com/go-resty/resty/v2"
	"golang.org/x/exp/slog"
	"path"
	"strconv"
	"strings"
)

const IDProperty = "id"

type Client struct {
	r      *resty.Client
	logger *slog.Logger
}

func WithBaseUrl(url string) func(client *resty.Client) *resty.Client {
	return func(client *resty.Client) *resty.Client {
		return client.SetBaseURL(url)
	}
}
func WithToken(token string) func(client *resty.Client) *resty.Client {
	return func(client *resty.Client) *resty.Client {
		return client.SetAuthToken(token)
	}
}

func NewClient(logger *slog.Logger, options ...func(client *resty.Client) *resty.Client) (Client, error) {
	lsClient := resty.New()
	for _, option := range options {
		lsClient = option(lsClient)
	}
	lsClient.HeaderAuthorizationKey = "Authorization"
	return Client{r: lsClient, logger: logger}, nil
}

func (c Client) CurrentGraph() (CurrentGraph, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.getCurrentGraph",
		"args":   nil,
	}).Post("")
	if err != nil {
		return CurrentGraph{}, err
	}
	if response.IsError() {
		return CurrentGraph{}, fmt.Errorf("error retrieving query: %s", string(response.Body()))
	}
	return UnmarshalCurrentGraph(response.Body())
}

func UnmarshalCurrentGraph(data []byte) (CurrentGraph, error) {
	var r CurrentGraph
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CurrentGraph) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type CurrentGraph struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func (c Client) Query(query string) (Query, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.q",
		"args":   []string{query},
	}).Post("")
	if err != nil {
		return nil, err
	}
	if response.IsError() {
		return nil, fmt.Errorf("error retrieving query: %s", string(response.Body()))
	}
	return UnmarshalQuery(response.Body())
}

type Query []Block

func UnmarshalQuery(data []byte) (Query, error) {
	var r Query
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Query) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Block struct {
	Properties           *Properties `json:"properties,omitempty"`
	Unordered            bool        `json:"unordered"`
	Parent               Left        `json:"parent"`
	ID                   int64       `json:"id"`
	PathRefs             []Left      `json:"pathRefs"`
	UUID                 string      `json:"uuid"`
	Content              string      `json:"content"`
	Children             []Block     `json:"children"`
	Page                 Page        `json:"page"`
	Left                 Left        `json:"left"`
	Format               string      `json:"format"`
	PropertiesTextValues *Properties `json:"propertiesTextValues,omitempty"`
	PropertiesOrder      []string    `json:"propertiesOrder"`
	Refs                 []Left      `json:"refs,omitempty"`
}

func UnmarshalBlock(data []byte) (Block, error) {
	var r Block
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Block) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Left struct {
	ID int64 `json:"id"`
}

type Page struct {
	UpdatedAt    int64  `json:"updatedAt"`
	JournalDay   int64  `json:"journalDay,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	UUID         string `json:"uuid"`
	Journal      bool   `json:"journal?,omitempty"`
	OriginalName string `json:"originalName"`
	File         Left   `json:"file"`
}

func UnmarshalPage(data []byte) (Page, error) {
	var r Page
	err := json.Unmarshal(data, &r)
	return r, err
}

var ErrInvalidPage = errors.New("invalid page")

func (r *Page) IsZero() bool {
	return r == nil || *r == Page{}
}

func (r *Page) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Page) ToURI(base string, journalPath string, pagePath string) (string, error) {
	if r.IsZero() {
		return "", ErrInvalidPage
	}
	sanitizedName := strings.Replace(r.OriginalName, "/", "___", -1)

	fileName := fmt.Sprintf("%s.md", sanitizedName)
	subFolder := pagePath
	if r.Journal {
		dateString := strconv.FormatInt(r.JournalDay, 10)
		fileName = fmt.Sprintf("%s_%s_%s.md", dateString[0:4], dateString[4:6], dateString[6:])
		subFolder = journalPath
	}
	return files.PathToFileURI(path.Join(base, subFolder, fileName)), nil
}

type Properties map[string]any

func (c Client) GetBlock(id string) (Block, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.getBlock",
		"args":   []any{id, map[string]bool{"includeChildren": true}},
	}).Post("")
	if err != nil {
		return Block{}, err
	}
	if response.IsError() {
		return Block{}, fmt.Errorf("error retrieving block: %s", string(response.Body()))
	}
	if len(response.Body()) == 0 || string(response.Body()) == "null" {
		return Block{}, errors.New("invalid response, ensure the logseq rest server running")
	}
	block, err := UnmarshalBlock(response.Body())
	if err != nil {
		c.logger.Error("error unmarshalling block", err, slog.String("raw", string(response.Body())))
		return Block{}, err
	}
	return block, err
}

func (c Client) GetPageById(id int64) (Page, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.getPage",
		"args":   []int64{id},
	}).Post("")
	if err != nil {
		return Page{}, err
	}
	if response.IsError() {
		return Page{}, fmt.Errorf("error retrieving block: %s", string(response.Body()))
	}
	if len(response.Body()) == 0 || string(response.Body()) == "null" {
		return Page{}, errors.New("invalid response, ensure the logseq rest server running")
	}
	return UnmarshalPage(response.Body())
}

func (c Client) GetPageByName(name string) (Page, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.getPage",
		"args":   []string{name},
	}).Post("")
	if err != nil {
		return Page{}, err
	}
	if response.IsError() {
		return Page{}, fmt.Errorf("error retrieving block: %s", string(response.Body()))
	}
	if len(response.Body()) == 0 || string(response.Body()) == "null" {
		return Page{}, errors.New("invalid response, ensure the logseq rest server running")
	}
	return UnmarshalPage(response.Body())
}

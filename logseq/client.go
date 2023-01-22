package logseq

import (
	"encoding/json"
	"fmt"
	"github.com/WhiskeyJack96/logseqlsp/files"
	"github.com/go-resty/resty/v2"
	"path"
	"strconv"
)

const IDProperty = "id"

type Client struct {
	r *resty.Client
}

func NewClient(options ...func()) (Client, error) {
	lsClient := resty.New().
		SetBaseURL("http://localhost:12315/api").
		SetAuthToken("test")
	lsClient.HeaderAuthorizationKey = "Authorization"
	return Client{r: lsClient}, nil
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
	Children             [][]*string `json:"children"`
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

func (r *Page) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Page) ToURI(base string, journalPath string, pagePath string) string {
	if r == nil {
		return ""
	}
	fileName := fmt.Sprintf("%s.md", r.Name)
	subFolder := pagePath
	if r.Journal {
		dateString := strconv.FormatInt(r.JournalDay, 10)
		fileName = fmt.Sprintf("%s_%s_%s.md", dateString[0:4], dateString[4:6], dateString[6:])
		subFolder = journalPath
	}
	return files.PathToFileURI(path.Join(base, subFolder, fileName))
}

type Properties map[string]any

func (c Client) GetBlock(id string) (Block, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.getBlock",
		"args":   []string{id},
	}).Post("")
	if err != nil {
		return Block{}, err
	}
	if response.IsError() {
		return Block{}, fmt.Errorf("error retrieving block: %s", string(response.Body()))
	}
	return UnmarshalBlock(response.Body())
}

func (c Client) GetPageById(id int64) (Page, error) {
	response, err := c.r.R().SetBody(map[string]any{
		"method": "logseq.App.getPage",
		"args":   []string{strconv.FormatInt(id, 10)},
	}).Post("")
	if err != nil {
		return Page{}, err
	}
	if response.IsError() {
		return Page{}, fmt.Errorf("error retrieving block: %s", string(response.Body()))
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
	return UnmarshalPage(response.Body())
}

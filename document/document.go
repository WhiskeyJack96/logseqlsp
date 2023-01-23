package document

import (
	"errors"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"io"
	"regexp"
	"strings"
)

type Document struct {
	Contents string
	Links    []Link
}

type Link struct {
	Target string
	Range  protocol.Range
	Type   linkType
}

type linkType string

var (
	Wiki       linkType = "WIKI"
	Tag        linkType = "TAG"
	Prop       linkType = "PROP"
	PropValue  linkType = "PROPVALUE"
	BlockEmbed linkType = "EMBED"
	Query      linkType = "QUERY"
)

var ErrLinkNotFound = errors.New("link not found")

var wikiLinkRegex = regexp.MustCompile(`(?:{{embed )?(\[*\[\[(.+?)]])`)
var queryLinkRegex = regexp.MustCompile(`{{query (.*)`)
var tagLinkRegex = regexp.MustCompile(`#([[:graph:]]+)[[:space:]]?`)
var propertyLinkRegex = regexp.MustCompile(`^[[:space:]]*-?[[:space:]]*((.*)::[[:space:]]*(.*))$`)
var embedLinkRegex = regexp.MustCompile(`.*\(?\(?([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})\)?\)?.*`)

// TODO add a document cache and update it on writes to avoid re-reading files every time an event happens
// TODO resolve all link uris at document load time to avoid re-querying the ls api
func New(reader io.Reader) (Document, error) {
	file, err := io.ReadAll(reader)
	if err != nil {
		return Document{}, err
	}

	var links []Link
	for line, content := range strings.Split(string(file), "\n") {

		//(0,1) start,end indexes of the regex match
		//(2,3) start,end indexes of the first capture
		//TODO order matters: make link regex not grab queries
		for _, match := range queryLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			links = append(links, newLink(href, Query, line, match[2], match[3]))
		}
		for _, match := range wikiLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[4]:match[5]]
			links = append(links, newLink(href, Wiki, line, match[2], match[3]))
		}
		for _, match := range tagLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			links = append(links, newLink(href, Tag, line, match[0], match[1]))
		}
		for _, match := range propertyLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[4]:match[5]]

			links = append(links, newLink(href, Prop, line, match[4], match[5]))

			//Value for id is technically a block embed link so we want to classify it as such
			if href != "id" {
				links = append(links, newLink(content[match[6]:match[7]], PropValue, line, match[6], match[7]))
			}

		}
		for _, match := range embedLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			links = append(links, newLink(href, BlockEmbed, line, match[2], match[3]))
		}

	}
	return Document{Contents: string(file), Links: links}, nil
}

func (d Document) FindLinkForPosition(pos protocol.Position) (Link, error) {
	for _, link := range d.Links {
		if positionInRange(d.Contents, link.Range, pos) {
			return link, nil
		}
	}
	return Link{}, ErrLinkNotFound
}

func newLink(href string, t linkType, line, start, end int) Link {
	if href == "" {
		return Link{}
	}

	//// Go regexes work with bytes, but the LSP client expects character indexes.
	//start = strutil.ByteIndexToRuneIndex(line, start)
	//end = strutil.ByteIndexToRuneIndex(line, end)
	return Link{
		Target: href,
		Type:   t,
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      protocol.UInteger(line),
				Character: protocol.UInteger(start),
			},
			End: protocol.Position{
				Line:      protocol.UInteger(line),
				Character: protocol.UInteger(end),
			},
		},
	}
}

func positionInRange(content string, rng protocol.Range, pos protocol.Position) bool {
	start, end := rng.IndexesIn(content)
	i := pos.IndexIn(content)
	//logger.Info("indexes", slog.Int("start", start), slog.Int("end", end), slog.Int("i", i))
	return i >= start && i <= end
}

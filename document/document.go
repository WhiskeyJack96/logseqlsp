package document

import (
	protocol "github.com/tliron/glsp/protocol_3_16"
	"golang.org/x/exp/slog"
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
	BlockEmbed linkType = "EMBED"
)

var wikiLinkRegex = regexp.MustCompile(`(?:{{embed )?\[*\[\[(.+?)]]`)
var tagLinkRegex = regexp.MustCompile(`#([[:graph:]]+)[[:space:]]?`)
var propertyLinkRegex = regexp.MustCompile(`^[[:space:]]*-?[[:space:]]*(.*)::(.*)$`)
var embedLinkRegex = regexp.MustCompile(`.*\(\(([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})\)\).*`)

func New(logger *slog.Logger, reader io.Reader) (Document, error) {
	file, err := io.ReadAll(reader)
	if err != nil {
		return Document{}, err
	}
	//logger.Info("parsing document", slog.String("content", string(file)))

	var links []Link
	for line, content := range strings.Split(string(file), "\n") {
		logger.Info("parsing document", slog.String("content", content))
		//(0,1) start,end indexes of the regex match
		//(2,3) start,end indexes of the first capture
		for _, match := range wikiLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			//logger.Info("found match", slog.Any("match", match))
			links = append(links, newLink(href, Wiki, line, match[0], match[1]))
		}
		for _, match := range tagLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			//logger.Info("found tag", slog.Any("match", match))
			links = append(links, newLink(href, Tag, line, match[0], match[1]))
		}
		for _, match := range propertyLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			//logger.Info("found prop", slog.Any("match", match))
			links = append(links, newLink(href, Prop, line, match[0], match[1]))
		}
		for _, match := range embedLinkRegex.FindAllStringSubmatchIndex(content, -1) {
			href := content[match[2]:match[3]]
			//logger.Info("found embed", slog.Any("match", match))
			links = append(links, newLink(href, BlockEmbed, line, match[0], match[1]))
		}
	}
	return Document{Contents: string(file), Links: links}, nil
}

func newLink(href string, t linkType, line, start, end int) Link {
	if href == "" {
		return Link{}
	}

	//// Go regexes work with bytes, but the LSP client expects character indexes.
	//start = strutil.ByteIndexToRuneIndex(line, start)
	//end = strutil.ByteIndexToRuneIndex(line, end)
	replace := strings.Replace(href, "/", "___", -1)
	switch t {
	case Wiki, Tag, Prop:
		replace = replace + ".md"
	}
	return Link{
		Target: replace,
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

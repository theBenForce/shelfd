package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ParsedAuthor represents an extracted book contributor.
type ParsedAuthor struct {
	Name string
	Role string
}

// ParsedSeries represents series information and sequence number.
type ParsedSeries struct {
	Name           string
	SequenceNumber *float64
}

// ParsedBook holds extracted metadata from an EPUB package.
type ParsedBook struct {
	Title                    string
	Authors                  []ParsedAuthor
	Genres                   []string
	Description              string
	Publisher                string
	Language                 string
	Identifier               string
	PublishedDate            string
	Series                   *ParsedSeries
	Layout                   string // "reflowable" or "pre-paginated"
	RenditionSpread          string // "auto", "landscape", "both", "none"
	RenditionOrientation     string // "auto", "landscape", "portrait"
	PageProgressionDirection string // "ltr" or "rtl"
}

// ParsedChapter holds extracted plaintext content and layout metadata for an EPUB spine section.
type ParsedChapter struct {
	Index        int
	Title        *string
	ContentPlain string
	Href         string
	PageWidth    *int
	PageHeight   *int
	PageSpread   *string
}

// Reader provides safe inspection and extraction methods for an EPUB archive.
type Reader struct {
	closer   io.Closer
	zip      *zip.Reader
	rootPath string
	rootBase string
	opf      *opfPackage
}

// Open opens an EPUB file on disk and verifies safety against Zip Slip attacks.
func Open(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening epub file: %w", err)
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stating epub file: %w", err)
	}

	r, err := zip.NewReader(f, fi.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("reading epub zip: %w", err)
	}

	reader, err := newReader(r, f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return reader, nil
}

// NewReader creates a Reader from an existing zip.Reader.
func NewReader(zr *zip.Reader) (*Reader, error) {
	return newReader(zr, nil)
}

func newReader(zr *zip.Reader, closer io.Closer) (*Reader, error) {
	// Guard against Zip Slip and path traversal
	for _, file := range zr.File {
		clean := filepath.Clean(file.Name)
		if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") ||
			strings.Contains(file.Name, "../") || strings.Contains(file.Name, `..\`) {
			return nil, fmt.Errorf("illegal path traversal detected in entry: %s", file.Name)
		}
	}

	reader := &Reader{
		closer: closer,
		zip:    zr,
	}

	// Resolve root package via META-INF/container.xml
	containerFile := reader.findFile("META-INF/container.xml")
	if containerFile == nil {
		return nil, fmt.Errorf("META-INF/container.xml not found in epub archive")
	}

	containerData, err := readZipFile(containerFile)
	if err != nil {
		return nil, fmt.Errorf("reading container.xml: %w", err)
	}

	rootPath, err := parseContainerXML(containerData)
	if err != nil {
		return nil, fmt.Errorf("parsing container.xml: %w", err)
	}
	reader.rootPath = rootPath
	reader.rootBase = path.Dir(rootPath)
	if reader.rootBase == "." {
		reader.rootBase = ""
	}

	// Parse root OPF
	opfFile := reader.findFile(rootPath)
	if opfFile == nil {
		return nil, fmt.Errorf("package file not found at %s", rootPath)
	}

	opfData, err := readZipFile(opfFile)
	if err != nil {
		return nil, fmt.Errorf("reading package file %s: %w", rootPath, err)
	}

	opf, err := parseOPF(opfData)
	if err != nil {
		return nil, fmt.Errorf("parsing opf package: %w", err)
	}
	reader.opf = opf

	return reader, nil
}

// Close closes the underlying reader if an io.Closer was supplied.
func (r *Reader) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// RootBase returns the base directory of the root OPF file inside the EPUB zip.
func (r *Reader) RootBase() string {
	return r.rootBase
}

// ReadEntry reads the raw bytes of an entry inside the EPUB zip by archive path.
func (r *Reader) ReadEntry(name string) ([]byte, error) {
	zf := r.findFile(name)
	if zf == nil {
		return nil, fmt.Errorf("file %s not found in epub", name)
	}
	return readZipFile(zf)
}

func (r *Reader) findFile(name string) *zip.File {
	normalized := path.Clean(name)
	for _, f := range r.zip.File {
		if path.Clean(f.Name) == normalized {
			return f
		}
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// 15MB limit per individual xml/text entry to protect against compression bombs
	limited := io.LimitReader(rc, 15*1024*1024)
	return io.ReadAll(limited)
}

type containerXML struct {
	XMLName   xml.Name `xml:"container"`
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

func parseContainerXML(data []byte) (string, error) {
	var c containerXML
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&c); err != nil {
		return "", err
	}

	for _, rf := range c.Rootfiles {
		if rf.FullPath != "" {
			return rf.FullPath, nil
		}
	}
	return "", fmt.Errorf("no valid rootfile found in container.xml")
}

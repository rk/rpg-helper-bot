package indexing

import (
	"fmt"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

// BookmarkDepthUnlimited imports every nested bookmark level (0).
const BookmarkDepthUnlimited = 0

// ExtractBookmarkSections builds sections from the PDF outline/bookmarks.
// maxDepth: 1 = top level only, 2 = two levels, etc.; 0 = unlimited.
func ExtractBookmarkSections(pdfPath string, maxDepth int) ([]models.TOCSection, int, error) {
	pageCount, err := PageCount(pdfPath)
	if err != nil {
		return nil, 0, err
	}

	bookmarks, err := readPDFBookmarks(pdfPath)
	if err != nil {
		return nil, pageCount, err
	}
	if len(bookmarks) == 0 {
		return nil, pageCount, fmt.Errorf("this PDF has no bookmarks/outline")
	}

	var entries []tocEntry
	flattenBookmarks(bookmarks, maxDepth, 1, &entries)
	entries = dedupeTOCEntries(entries)
	if len(entries) == 0 {
		return nil, pageCount, fmt.Errorf("no usable bookmarks with page numbers were found")
	}
	return BuildTOCSections(entries, pageCount), pageCount, nil
}

func readPDFBookmarks(pdfPath string) ([]pdfcpu.Bookmark, error) {
	f, err := os.Open(pdfPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		bookmarks []pdfcpu.Bookmark
		readErr   error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				readErr = fmt.Errorf("bookmark read failed: %v", r)
			}
		}()
		bookmarks, readErr = api.Bookmarks(f, model.NewDefaultConfiguration())
	}()
	if readErr != nil {
		return nil, readErr
	}
	return bookmarks, nil
}

func flattenBookmarks(bookmarks []pdfcpu.Bookmark, maxDepth, depth int, out *[]tocEntry) {
	for _, bm := range bookmarks {
		title := strings.TrimSpace(bm.Title)
		if title != "" && bm.PageFrom >= 1 {
			*out = append(*out, tocEntry{Title: title, StartPage: bm.PageFrom})
		}
		if len(bm.Kids) == 0 {
			continue
		}
		if maxDepth > 0 && depth >= maxDepth {
			continue
		}
		flattenBookmarks(bm.Kids, maxDepth, depth+1, out)
	}
}

// ResolveBookmarkMaxDepth maps API options to a bookmark depth (1+ or 0 = unlimited).
func ResolveBookmarkMaxDepth(maxDepth *int, includeChildren *bool) int {
	if maxDepth != nil {
		if *maxDepth < 0 {
			return BookmarkDepthUnlimited
		}
		if *maxDepth == 0 {
			return BookmarkDepthUnlimited
		}
		return *maxDepth
	}
	if includeChildren != nil && !*includeChildren {
		return 1
	}
	return BookmarkDepthUnlimited
}

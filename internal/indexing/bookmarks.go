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

// ExtractBookmarkSections builds sections from the PDF outline/bookmarks.
func ExtractBookmarkSections(pdfPath string, includeChildren bool) ([]models.TOCSection, int, error) {
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
	flattenBookmarks(bookmarks, includeChildren, &entries)
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

func flattenBookmarks(bookmarks []pdfcpu.Bookmark, includeChildren bool, out *[]tocEntry) {
	for _, bm := range bookmarks {
		title := strings.TrimSpace(bm.Title)
		if title != "" && bm.PageFrom >= 1 {
			*out = append(*out, tocEntry{Title: title, StartPage: bm.PageFrom})
		}
		if includeChildren && len(bm.Kids) > 0 {
			flattenBookmarks(bm.Kids, true, out)
		}
	}
}

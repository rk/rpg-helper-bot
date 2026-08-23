package indexing

// Indexer will extract PDF text and build vector indexes in a later phase.
type Indexer interface {
	IndexPDF(pdfID string) error
}

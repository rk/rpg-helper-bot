package models

import "time"

type IndexStatus string

const (
	IndexStatusNone      IndexStatus = "none"
	IndexStatusIndexing  IndexStatus = "indexing"
	IndexStatusIndexed   IndexStatus = "indexed"
	IndexStatusError     IndexStatus = "error"
)

type Game struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	Archived  bool      `json:"archived"`
	Running   bool      `json:"running"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PDFIndexMeta struct {
	Features           []PDFFeature      `json:"features"`
	Glossary           []PDFGlossary     `json:"glossary"`
	Cheatsheet         []CheatsheetEntry `json:"cheatsheet"`
	LLMGlossarySkipped bool              `json:"llm_glossary_skipped,omitempty"`
}

type PDFFeature struct {
	FeatureID string   `json:"feature_id"`
	Sections  []string `json:"sections"`
	Terms     []string `json:"terms"`
}

type PDFGlossary struct {
	FeatureID string `json:"feature_id"`
	PDFTerm   string `json:"pdf_term"`
	Evidence  string `json:"evidence,omitempty"`
}

type CheatsheetEntry struct {
	FeatureID   string   `json:"feature_id"`
	FeatureName string   `json:"feature_name,omitempty"`
	PDFTerms    []string `json:"pdf_terms"`
	Section     string   `json:"section"`
	StartPage   int      `json:"start_page"`
}

type PDF struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	FilePath      string      `json:"file_path"`
	PageCount     int         `json:"page_count"`
	ThumbnailPath string      `json:"thumbnail_path,omitempty"`
	IndexStatus   IndexStatus `json:"index_status"`
	IndexedAt     *time.Time  `json:"indexed_at,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type TOCSection struct {
	ID        string `json:"id"`
	PDFID     string `json:"pdf_id"`
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
	SortOrder int    `json:"sort_order"`
	PlainText string `json:"plain_text,omitempty"`
	Indexed   bool   `json:"indexed"`
}

type GamePDF struct {
	PDFID     string `json:"pdf_id"`
	SortOrder int    `json:"sort_order"`
}

type PathStatus string

const (
	PathStatusOK      PathStatus = "ok"
	PathStatusMissing PathStatus = "missing"
)

type SearchHit struct {
	SectionID    string  `json:"section_id"`
	PDFID        string  `json:"pdf_id"`
	PDFTitle     string  `json:"pdf_title"`
	SectionTitle string  `json:"section_title"`
	StartPage    int     `json:"start_page"`
	EndPage      int     `json:"end_page"`
	PDFSortOrder int     `json:"pdf_sort_order"`
	Score        float64 `json:"score"`
	FTSRank      float64 `json:"fts_rank,omitempty"`
	Snippet      string  `json:"snippet"`
}

type ChatSearchDebug struct {
	OriginalQuery     string           `json:"original_query"`
	FTSQuery          string           `json:"fts_query"`
	QueryRewritten    bool             `json:"query_rewritten"`
	RewriteError      string           `json:"rewrite_error,omitempty"`
	FTSCandidateCount int              `json:"fts_candidate_count"`
	LLMConfigured     bool             `json:"llm_configured"`
	Hits              []ChatSearchHit  `json:"hits"`
}

type ChatSearchHit struct {
	Rank         int     `json:"rank"`
	SectionID    string  `json:"section_id"`
	PDFTitle     string  `json:"pdf_title"`
	SectionTitle string  `json:"section_title"`
	StartPage    int     `json:"start_page"`
	EndPage      int     `json:"end_page"`
	PDFSortOrder int     `json:"pdf_sort_order"`
	EmbedScore   float64 `json:"embed_score"`
	FTSRank      float64 `json:"fts_rank"`
	Snippet      string  `json:"snippet,omitempty"`
}

type ChatSource struct {
	SectionID    string `json:"section_id"`
	PDFTitle     string `json:"pdf_title"`
	SectionTitle string `json:"section_title"`
	StartPage    int    `json:"start_page"`
	EndPage      int    `json:"end_page"`
	Snippet      string `json:"snippet,omitempty"`
}

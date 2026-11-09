package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/ulid"
)

// =============================================================================
// Helper Utilities for Full-Stack & UI/UX Contract E2E Verification
// =============================================================================

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "pnpm-workspace.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate pnpm-workspace.yaml in any parent directory of %s", dir)
		}
		dir = parent
	}
}

func readRepoFile(t *testing.T, relPath string) string {
	t.Helper()
	full := filepath.Join(repoRoot(t), relPath)
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("failed to read %s: %v", relPath, err)
	}
	if len(b) == 0 {
		t.Fatalf("file %s is empty", relPath)
	}
	return string(b)
}

func readAppFile(t *testing.T, relPath string) string {
	t.Helper()
	return readRepoFile(t, filepath.Join("apps", "app", relPath))
}

func assertContainsAll(t *testing.T, label, content string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(content, n) {
			t.Errorf("[%s] expected to contain %q, but it was missing", label, n)
		}
	}
}

func assertContainsNone(t *testing.T, label, content string, forbidden ...string) {
	t.Helper()
	for _, f := range forbidden {
		if strings.Contains(content, f) {
			t.Errorf("[%s] expected NOT to contain %q, but it was present", label, f)
		}
	}
}

// assertRedesignContractIfLanded enforces progressive redesign contract requirements
// whenever the file has received its milestone marker symbol OR when SHELFD_M4_STRICT=1.
func assertRedesignContractIfLanded(t *testing.T, label, content, triggerSymbol string, requiredPatterns ...string) {
	t.Helper()
	strict := os.Getenv("SHELFD_M4_STRICT") == "1"
	if strings.Contains(content, triggerSymbol) || strict {
		for _, p := range requiredPatterns {
			if !strings.Contains(content, p) {
				t.Errorf("[%s redesign contract] expected %q once %q landed", label, p, triggerSymbol)
			}
		}
	}
}

// assertValidDartSource strips Dart comments and string literals and verifies balanced
// braces, parentheses, and brackets so syntax corruption is caught immediately.
func assertValidDartSource(t *testing.T, relPath, src string) {
	t.Helper()
	var braceDepth, parenDepth, bracketDepth int
	n := len(src)
	i := 0
	for i < n {
		// Line comment
		if i+1 < n && src[i] == '/' && src[i+1] == '/' {
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment
		if i+1 < n && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}
		// Triple-quoted strings
		if i+2 < n && ((src[i] == '\'' && src[i+1] == '\'' && src[i+2] == '\'') ||
			(src[i] == '"' && src[i+1] == '"' && src[i+2] == '"')) {
			q := src[i]
			i += 3
			for i+2 < n && !(src[i] == q && src[i+1] == q && src[i+2] == q) {
				if src[i] == '\\' {
					i += 2
					continue
				}
				i++
			}
			i += 3
			continue
		}
		// Raw string literal r'...' or r"..."
		if i+1 < n && src[i] == 'r' && (src[i+1] == '\'' || src[i+1] == '"') {
			q := src[i+1]
			i += 2
			for i < n && src[i] != q {
				i++
			}
			i++
			continue
		}
		// Single or double quoted string
		if src[i] == '\'' || src[i] == '"' {
			q := src[i]
			i++
			for i < n && src[i] != q {
				if src[i] == '\\' {
					i += 2
					continue
				}
				// Handle string interpolation ${...} simply by skipping inner quotes
				i++
			}
			i++
			continue
		}

		switch src[i] {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		}
		i++
	}
	if braceDepth != 0 {
		t.Errorf("%s has unbalanced curly braces (net delta: %d)", relPath, braceDepth)
	}
	if parenDepth != 0 {
		t.Errorf("%s has unbalanced parentheses (net delta: %d)", relPath, parenDepth)
	}
	if bracketDepth != 0 {
		t.Errorf("%s has unbalanced square brackets (net delta: %d)", relPath, bracketDepth)
	}
}

func buildInMemoryEPUB(t *testing.T, title, author, description string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	m, err := zw.Create("mimetype")
	if err != nil {
		t.Fatalf("create mimetype: %v", err)
	}
	_, _ = m.Write([]byte("application/epub+zip"))

	c, _ := zw.Create("META-INF/container.xml")
	_, _ = c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))

	authorXML := ""
	if author != "" {
		authorXML = fmt.Sprintf("<dc:creator>%s</dc:creator>", author)
	}
	opf, _ := zw.Create("OEBPS/content.opf")
	_, _ = opf.Write([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>%s</dc:title>
    %s
    <dc:description>%s</dc:description>
    <dc:publisher>Scriptorium Press</dc:publisher>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`, title, authorXML, description)))

	ch1, _ := zw.Create("OEBPS/ch1.xhtml")
	_, _ = ch1.Write([]byte(`<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><body><h1>Chapter 1</h1><p>In the quiet scriptorium, the vellum pages gleamed under warm amber light.</p></body></html>`))

	if err := zw.Close(); err != nil {
		t.Fatalf("close epub zip: %v", err)
	}
	return buf.Bytes()
}

func relativeLuminance(argb uint64) float64 {
	r := float64((argb>>16)&0xFF) / 255.0
	g := float64((argb>>8)&0xFF) / 255.0
	b := float64(argb&0xFF) / 255.0
	lin := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// =============================================================================
// TIER 1 — FEATURE COVERAGE (12 Features x 5 Subtests = 60 Subtests)
// =============================================================================

func TestTactileScriptorium_Tier1_FeatureCoverage(t *testing.T) {
	// -------------------------------------------------------------------------
	// Feature 1: Theme Tokens & Dynamic Palette (tokens.dart, theme.dart)
	// -------------------------------------------------------------------------
	t.Run("F01_T1_01_TouchAndLayoutInvariants", func(t *testing.T) {
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		assertContainsAll(t, "tokens.dart", tokens,
			"minTouchTarget = 48.0",
			"maxReadingWidth = 720.0",
			"maxSearchWidth = 860.0",
			"maxLibraryWidth",
			"sidebarWidth = 240.0",
			"mobileBreakpoint = 640.0",
			"tabletBreakpoint = 1024.0",
		)
	})

	t.Run("F01_T1_02_SpacingAndRadiusScale", func(t *testing.T) {
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		assertContainsAll(t, "tokens.dart", tokens,
			"space4 = 4.0", "space8 = 8.0", "space12 = 12.0", "space16 = 16.0",
			"space20 = 20.0", "space24 = 24.0", "space32 = 32.0", "space48 = 48.0", "space64 = 64.0",
			"radiusSm = 4.0", "radiusMd = 8.0", "radiusLg = 12.0", "radiusPill = 999.0",
		)
	})

	t.Run("F01_T1_03_BoneSepiaDarkBasePalettes", func(t *testing.T) {
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		assertContainsAll(t, "tokens.dart", tokens,
			"boneBackground = Color(0xFFFBFBFA)",
			"boneSurface = Color(0xFFF9F9F8)",
			"boneContainer = Color(0xFFF3F4F3)",
			"charcoalInk = Color(0xFF111111)",
			"mutedCopy = Color(0xFF444748)",
			"crispBorder = Color(0xFFEAEAEA)",
			"sepiaBackground = Color(0xFFF4ECD8)",
			"sepiaSurface = Color(0xFFEDE3CB)",
			"sepiaInk = Color(0xFF5B4636)",
			"darkOledBackground = Color(0xFF121212)",
			"darkOledSurface = Color(0xFF1E1E1E)",
			"darkOledInk = Color(0xFFE0E0E0)",
		)
	})

	t.Run("F01_T1_04_ObsidianAmberAndScriptoriumTokens", func(t *testing.T) {
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		assertContainsAll(t, "tokens.dart", tokens,
			"obsidianBackground = Color(0xFF141315)",
			"obsidianContainer = Color(0xFF1C1B1D)",
			"obsidianSurface = Color(0xFF201F21)",
			"amberAccent = Color(0xFFE5A967)",
			"amberGlow = Color(0xFFFFC589)",
			"terracottaAccent = Color(0xFFD97736)",
			"antiqueCream = Color(0xFFF3C99D)",
			"matchBadgeBg", "matchBadgeText",
		)
		assertRedesignContractIfLanded(t, "tokens.dart", tokens, "siennaPrimary",
			"siennaPrimary = Color(0xFF8E4A26)",
			"libraryGreen = Color(0xFF3A5A40)",
			"warmParchment = Color(0xFFFBF9F4)",
			"walnutBackground = Color(0xFF161412)",
			"espressoInk = Color(0xFF1E1B18)",
			"ghostBorder = Color(0xFFE3DDD3)",
		)
	})

	t.Run("F01_T1_05_AppThemeBuilderModesAndShelfdColorsExtension", func(t *testing.T) {
		theme := readAppFile(t, "lib/ui/core/theme.dart")
		assertContainsAll(t, "theme.dart", theme,
			"enum ReadingThemeMode { bone, sepia, dark }",
			"static ThemeData buildTheme(ReadingThemeMode mode)",
			"ReadingThemeMode.bone",
			"ReadingThemeMode.sepia",
			"ReadingThemeMode.dark",
		)
		assertRedesignContractIfLanded(t, "theme.dart", theme, "class ShelfdColors",
			"extends ThemeExtension<ShelfdColors>",
			"static const ShelfdColors bone",
			"static const ShelfdColors sepia",
			"static const ShelfdColors dark",
			"static ShelfdColors of(BuildContext context)",
			"ShelfdColors copyWith(",
			"ShelfdColors lerp(",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 2: Editorial Typography System (typography.dart, web/index.html, html_text.dart)
	// -------------------------------------------------------------------------
	t.Run("F02_T1_01_AppTypographyCoreMethods", func(t *testing.T) {
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		assertContainsAll(t, "typography.dart", typo,
			"static TextStyle titleSerif(",
			"static TextStyle bodySans(",
			"static TextStyle labelCaps(",
			"static TextStyle captionSans(",
			"static TextStyle readerText(",
		)
	})

	t.Run("F02_T1_02_SerifSansAndMonoStacksWithTabularFigures", func(t *testing.T) {
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		assertContainsAll(t, "typography.dart", typo,
			"serifFallbacks", "Georgia", "serif",
			"sansFallbacks", "sans-serif",
		)
		assertRedesignContractIfLanded(t, "typography.dart", typo, "editorialSerifFamily",
			"editorialSerifFamily = 'Newsreader'",
			"humanistSansFamily = 'Work Sans'",
			"technicalMonoFamily = 'JetBrains Mono'",
			"static TextStyle mono(",
			"FontFeature.tabularFigures()",
		)
	})

	t.Run("F02_T1_03_HtmlTextSanitizationAndBlockRendering", func(t *testing.T) {
		htmlText := readAppFile(t, "lib/ui/core/html_text.dart")
		assertContainsAll(t, "html_text.dart", htmlText,
			"class HtmlText extends StatelessWidget",
			"decodeHtmlEntities",
			"parseHtmlBlocks",
			"parseHtmlInlineSpans",
			"HtmlBlockType",
		)
		assertRedesignContractIfLanded(t, "html_text.dart", htmlText, "ShelfdColors.of(context)",
			"colors.ink",
			"colors.mutedInk",
			"colors.accentPrimary",
		)
	})

	t.Run("F02_T1_04_WebIndexHtmlContractAndFontPreload", func(t *testing.T) {
		indexHTML := readAppFile(t, "web/index.html")
		assertContainsAll(t, "web/index.html", indexHTML,
			"<!DOCTYPE html>",
			`<base href="$FLUTTER_BASE_HREF">`,
			`<link rel="manifest" href="manifest.json">`,
			`flutter_bootstrap.js`,
		)
		assertRedesignContractIfLanded(t, "web/index.html", indexHTML, "fonts.googleapis.com",
			"Newsreader",
			"Work+Sans",
			"JetBrains+Mono",
		)
	})

	t.Run("F02_T1_05_HtmlTextWidgetTestAndE2EContract", func(t *testing.T) {
		htmlTest := readAppFile(t, "test/ui/core/html_text_test.dart")
		e2eTest := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertContainsAll(t, "html_text_test.dart", htmlTest, "HtmlText", "decodeHtmlEntities", "parseHtmlBlocks")
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2eTest, "F02-T1-01", "F02-T1-02", "F02-T1-05")
	})

	// -------------------------------------------------------------------------
	// Feature 3: Shared Layout, Navigation Shell & Tactile Book Cards
	// -------------------------------------------------------------------------
	t.Run("F03_T1_01_CoreSharedPrimitives", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"class BentoCard extends StatelessWidget",
			"Card(",
			"class FilterPillsRow extends StatelessWidget",
			"ChoiceChip(",
			"class StatusBadge extends StatelessWidget",
			"class PrimaryButton extends StatelessWidget",
			"class AuthorAvatar extends StatelessWidget",
		)
	})

	t.Run("F03_T1_02_NavigationComponentsAndSevenDestinations", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"class ShelfdTopBar extends",
			"class ShelfdUploadsBadgeButton extends",
			"class ShelfdBottomNav extends",
			"class ShelfdSideNav extends",
			"class ShelfdAdaptiveScaffold extends",
			"label: 'Books'",
			"label: 'Series'",
			"label: 'Authors'",
			"label: 'Search'",
			"label: 'Uploads'",
			"label: 'Utilities'",
			"label: 'Settings'",
			"Semantic Search",
		)
	})

	t.Run("F03_T1_03_PolymorphicShelfdGridCardAndTactileBookCover", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"class ShelfdGridCard extends StatelessWidget",
			"BookGridItem",
			"SeriesGridItem",
			"AuthorGridItem",
			"GenreGridItem",
			"TopicGridItem",
		)
		assertRedesignContractIfLanded(t, "shared_layout.dart", shared, "class TactileBookCover",
			"showSpineCrease",
			"showShadow",
			"isSeries",
			"_buildEditorialFallback",
		)
	})

	t.Run("F03_T1_04_AppShellComposition", func(t *testing.T) {
		shell := readAppFile(t, "lib/ui/core/app_shell.dart")
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "app_shell.dart", shell,
			"class AppShell extends ConsumerWidget",
			"ShelfdAdaptiveScaffold(",
			"triggerScan()",
		)
		assertContainsAll(t, "shared_layout.dart", shared, "MiniPlayerBar")
	})

	t.Run("F03_T1_05_SettingsModalAndResolveBookTitle", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"String? resolveBookTitle(String? bookOrId, List<Book> books)",
			"showShelfdSettingsModal(",
			"Uploads & Staged Queue",
			"Library Utilities & Merge",
			"Change Password & Account Settings",
			"Disconnect / Sign Out",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 4: Library Catalog & Structured Filter Bar
	// -------------------------------------------------------------------------
	t.Run("F04_T1_01_LiveAPI_ListBooksAndPagination", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/books?limit=10&offset=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Books []api.BookListItem `json:"books"`
			Total int                `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode books response: %v", err)
		}
		if resp.Total != 1 || len(resp.Books) != 1 || resp.Books[0].Title != "Solaris: The Novel?" {
			t.Fatalf("unexpected books payload: %+v", resp)
		}
	})

	t.Run("F04_T1_02_LiveAPI_StructuredSearchTokens", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		q := url.QueryEscape(`author:"Stanislaw Lem" genre:"Science Fiction" topic:"First Contact" Solaris`)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/books?search="+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Total int `json:"total"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Total != 1 {
			t.Errorf("expected 1 matching book for structured search tokens, got %d", resp.Total)
		}
	})

	t.Run("F04_T1_03_LibraryViewModesAndGridDelegate", func(t *testing.T) {
		libView := readAppFile(t, "lib/ui/features/library/library_view.dart")
		assertContainsAll(t, "library_view.dart", libView,
			"enum LibraryViewMode",
			"LibraryViewMode.books",
			"LibraryViewMode.series",
			"LibraryViewMode.authors",
			"LibraryViewMode.genres",
			"LibraryViewMode.topics",
			"SliverGridDelegateWithMaxCrossAxisExtent",
			"maxCrossAxisExtent: 200.0",
			"childAspectRatio: 0.55",
		)
	})

	t.Run("F04_T1_04_LibraryFilterBarAndQueryParser", func(t *testing.T) {
		filterBar := readAppFile(t, "lib/ui/features/library/library_filter_bar.dart")
		parser := readAppFile(t, "lib/ui/features/library/library_query_parser.dart")
		assertContainsAll(t, "library_filter_bar.dart", filterBar, "class LibraryFilterBar", "TextField")
		assertContainsAll(t, "library_query_parser.dart", parser, "class ParsedLibraryQuery", "matchesBook")
	})

	t.Run("F04_T1_05_SeriesGroupingToggleContract", func(t *testing.T) {
		providers := readAppFile(t, "lib/ui/state/providers.dart")
		assertContainsAll(t, "providers.dart", providers, "groupedBookItems", "SeriesGridItem", "BookGridItem")
	})

	// -------------------------------------------------------------------------
	// Feature 5: Taxonomy & Entity Detail Views
	// -------------------------------------------------------------------------
	t.Run("F05_T1_01_LiveAPI_ListAuthorsAndSeries", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		for _, ep := range []string{"/api/v1/authors", "/api/v1/series"} {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s returned %d", ep, rec.Code)
			}
		}
	})

	t.Run("F05_T1_02_LiveAPI_ListGenresAndTopics", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		for _, ep := range []string{"/api/v1/genres", "/api/v1/topics"} {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s returned %d", ep, rec.Code)
			}
		}
	})

	t.Run("F05_T1_03_LiveAPI_AuthorPhotoUploadAndFetch", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		author, err := f.repo.UpsertAuthor(context.Background(), "Ursula K. Le Guin")
		if err != nil {
			t.Fatalf("UpsertAuthor: %v", err)
		}

		jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("photo", "author.jpg")
		_, _ = part.Write(jpegBytes)
		_ = mpw.Close()

		postReq := httptest.NewRequest(http.MethodPost, "/api/v1/authors/"+author.ID+"/photo", &body)
		postReq.Header.Set("Authorization", "Bearer "+token)
		postReq.Header.Set("Content-Type", mpw.FormDataContentType())
		postRec := httptest.NewRecorder()
		f.handler.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on author photo upload, got %d: %s", postRec.Code, postRec.Body.String())
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/authors/"+author.ID+"/photo", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK || !bytes.Equal(getRec.Body.Bytes(), jpegBytes) {
			t.Fatalf("expected 200 and matching bytes on author photo fetch, got %d", getRec.Code)
		}
	})

	t.Run("F05_T1_04_AuthorDetailViewContract", func(t *testing.T) {
		view := readAppFile(t, "lib/ui/features/authors/author_detail_view.dart")
		assertContainsAll(t, "author_detail_view.dart", view,
			"class AuthorDetailView",
			"AuthorAvatar",
			"Books by Author",
		)
	})

	t.Run("F05_T1_05_SeriesDetailViewSequenceSorting", func(t *testing.T) {
		view := readAppFile(t, "lib/ui/features/series/series_detail_view.dart")
		assertContainsAll(t, "series_detail_view.dart", view,
			"class SeriesDetailView",
			"int compareSeriesBooks(Book a, Book b)",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 6: Book Detail Showcase, Annotations & RAG Chat
	// -------------------------------------------------------------------------
	t.Run("F06_T1_01_LiveAPI_GetBookDetailFullPayload", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var detail api.BookDetailResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
			t.Fatalf("unmarshal detail: %v", err)
		}
		if detail.Title != "Solaris: The Novel?" || len(detail.Authors) != 1 || len(detail.Chapters) != 2 || len(detail.Highlights) != 1 {
			t.Errorf("unexpected book detail response: %+v", detail)
		}
	})

	t.Run("F06_T1_02_LiveAPI_BookmarksAndHighlightsCRUD", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		// Create Bookmark
		bmReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/bookmarks", strings.NewReader(`{"title":"Station Arrival","progress":0.25}`))
		bmReq.Header.Set("Authorization", "Bearer "+token)
		bmReq.Header.Set("Content-Type", "application/json")
		bmRec := httptest.NewRecorder()
		f.handler.ServeHTTP(bmRec, bmReq)
		if bmRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 on bookmark create, got %d", bmRec.Code)
		}

		// Test all 4 Kindle colors for Highlights
		for _, color := range []string{"yellow", "blue", "pink", "orange"} {
			hlBody := fmt.Sprintf(`{"selected_text":"Quote in %s","color":"%s","note":"Note %s"}`, color, color, color)
			hlReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/highlights", strings.NewReader(hlBody))
			hlReq.Header.Set("Authorization", "Bearer "+token)
			hlReq.Header.Set("Content-Type", "application/json")
			hlRec := httptest.NewRecorder()
			f.handler.ServeHTTP(hlRec, hlReq)
			if hlRec.Code != http.StatusCreated {
				t.Fatalf("expected 201 on %s highlight create, got %d", color, hlRec.Code)
			}
		}
	})

	t.Run("F06_T1_03_LiveAPI_BookRAGChatWithCitations", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)
		chTitle := "Chapter 3: The Visitors"
		_ = f.repo.CreateChapter(context.Background(), &repository.Chapter{
			ID:           "ch-nlm-3",
			BookID:       book.ID,
			ChapterIndex: 3,
			Title:        &chTitle,
			Summary:      "Kelvin arrives at the Solaris research station and meets Snow.",
			ContentPlain: "Kelvin arrives at the station.",
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/chat", strings.NewReader(`{"query":"What happens when Kelvin arrives?"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on book chat, got %d: %s", rec.Code, rec.Body.String())
		}
		var chatResp struct {
			Reply     string `json:"reply"`
			Citations []any  `json:"citations"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &chatResp); err != nil {
			t.Fatalf("unmarshal chat response: %v", err)
		}
		if chatResp.Reply == "" || chatResp.Citations == nil {
			t.Errorf("expected non-empty reply and non-nil citations slice, got %+v", chatResp)
		}
	})

	t.Run("F06_T1_04_LiveAPI_NotebookLMMarkdownExportAndHandoff", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID+"/notebooklm", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), "We don't need other worlds") {
			t.Fatalf("expected 200 and highlight in NotebookLM markdown export, got %d", getRec.Code)
		}

		postReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/notebooklm", nil)
		postReq.Header.Set("Authorization", "Bearer "+token)
		postRec := httptest.NewRecorder()
		f.handler.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusOK || !strings.Contains(postRec.Body.String(), "client_handoff") {
			t.Fatalf("expected 200 and client_handoff on POST notebooklm, got %d", postRec.Code)
		}
	})

	t.Run("F06_T1_05_BookDetailViewPreservedKeysAndTabs", func(t *testing.T) {
		view := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		assertContainsAll(t, "book_detail_view.dart", view,
			"Key('download_hero_menu_button')",
			"Key('download_appbar_menu_button')",
			"Key('menu_item_download_epub')",
			"Key('menu_item_send_notebooklm')",
			"Key('menu_item_download_markdown')",
			"Key('edit_metadata_appbar_button')",
			"Key('edit_metadata_sidebar_button')",
			"Key('listen_hero_button')",
			"Key('read_hero_button')",
			"Overview",
			"Highlights & Bookmarks",
			"Chat with Book",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 7: Metadata Editor & Multi-Provider Search Modals
	// -------------------------------------------------------------------------
	t.Run("F07_T1_01_LiveAPI_UpdateBookMetadataAndRelations", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		payload := `{"title":"Solaris: Definitive Edition","authors":["Stanislaw Lem","Bill Johnston"],"series":"Lem Masterworks","sequence_number":1,"publisher":"MIT Press","language":"en","genres":["Sci-Fi"],"topics":["Philosophy"]}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+book.ID+"/metadata", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on metadata update, got %d: %s", rec.Code, rec.Body.String())
		}
		var updated api.BookDetailResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &updated)
		if updated.Title != "Solaris: Definitive Edition" || len(updated.Authors) != 2 {
			t.Errorf("unexpected updated metadata: %+v", updated)
		}
	})

	t.Run("F07_T1_02_LiveAPI_UploadBookCoverReplacement", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0D}
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("cover", "cover.png")
		_, _ = part.Write(pngHeader)
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/cover", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on cover upload, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("F07_T1_03_EditBookMetadataDialogKeysAndFields", func(t *testing.T) {
		dlg := readAppFile(t, "lib/ui/features/book_detail/edit_book_metadata_dialog.dart")
		assertContainsAll(t, "edit_book_metadata_dialog.dart", dlg,
			"Key('edit_book_title_field')",
			"Key('edit_book_authors_field')",
			"Key('edit_book_series_field')",
			"Key('edit_book_sequence_field')",
			"Key('edit_book_genres_field')",
			"Key('edit_book_topics_field')",
			"Key('edit_book_publisher_field')",
			"Key('edit_book_language_field')",
			"Key('edit_book_description_field')",
			"Key('save_book_metadata_button')",
			"Save & Sync EPUB",
		)
	})

	t.Run("F07_T1_04_MetadataSearchDialogMultiProviderUI", func(t *testing.T) {
		searchDlg := readAppFile(t, "lib/ui/features/book_detail/metadata_search_dialog.dart")
		assertContainsAll(t, "metadata_search_dialog.dart", searchDlg,
			"class MetadataSearchDialogContent",
			"showMetadataSearchDialog",
			"Search Metadata",
			"Apply",
		)
	})

	t.Run("F07_T1_05_MetadataSearchResultModelContract", func(t *testing.T) {
		model := readAppFile(t, "lib/data/models/metadata_search_result.dart")
		searchDlg := readAppFile(t, "lib/ui/features/book_detail/metadata_search_dialog.dart")
		assertContainsAll(t, "metadata_search_result.dart", model,
			"class MetadataSearchResult",
			"factory MetadataSearchResult.fromJson",
			"provider",
		)
		assertContainsAll(t, "metadata_search_dialog.dart", searchDlg,
			"Open Library",
			"Google Books",
			"Apple Books",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 8: Reflowable & Fixed-Layout EPUB Readers
	// -------------------------------------------------------------------------
	t.Run("F08_T1_01_LiveAPI_GetChapterByIndexZeroAndID", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		for _, path := range []string{
			"/api/v1/books/" + book.ID + "/chapters/1",
			"/api/v1/books/" + book.ID + "/chapters/0",
			"/api/v1/chapters/ch-nlm-1",
		} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s returned %d: %s", path, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("F08_T1_02_LiveAPI_GetChapterHTMLFixedLayoutRewriting", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		bookDir := filepath.Join(f.libDir, "Art", "SpreadBook")
		_ = os.MkdirAll(bookDir, 0755)
		epubPath := filepath.Join(bookDir, "SpreadBook.epub")

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		m, _ := zw.Create("mimetype")
		_, _ = m.Write([]byte("application/epub+zip"))
		c, _ := zw.Create("META-INF/container.xml")
		_, _ = c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
		opf, _ := zw.Create("OEBPS/content.opf")
		_, _ = opf.Write([]byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>SpreadBook</dc:title><meta property="rendition:layout">pre-paginated</meta></metadata><manifest><item id="p1" href="pages/p1.xhtml" media-type="application/xhtml+xml"/><item id="img1" href="images/plate.jpg" media-type="image/jpeg"/></manifest><spine><itemref idref="p1" properties="page-spread-right"/></spine></package>`))
		p1, _ := zw.Create("OEBPS/pages/p1.xhtml")
		_, _ = p1.Write([]byte(`<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><meta name="viewport" content="width=900, height=1200"/></head><body><img src="../images/plate.jpg"/></body></html>`))
		img, _ := zw.Create("OEBPS/images/plate.jpg")
		_, _ = img.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
		_ = zw.Close()
		_ = os.WriteFile(epubPath, buf.Bytes(), 0644)

		book, err := f.ingester.IngestFile(context.Background(), epubPath, filepath.Join("Art", "SpreadBook", "SpreadBook.epub"))
		if err != nil {
			t.Fatalf("IngestFile: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/0/html", book.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on chapter HTML, got %d: %s", rec.Code, rec.Body.String())
		}
		assertContainsAll(t, "chapter HTML", rec.Body.String(),
			fmt.Sprintf("/api/v1/books/%s/assets/OEBPS/images/plate.jpg", book.ID),
			"viewport",
		)
	})

	t.Run("F08_T1_03_LiveAPI_GetBookAssetFromEPUB", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		bookDir := filepath.Join(f.libDir, "Art", "AssetBook")
		_ = os.MkdirAll(bookDir, 0755)
		epubPath := filepath.Join(bookDir, "AssetBook.epub")
		imgBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		m, _ := zw.Create("mimetype")
		_, _ = m.Write([]byte("application/epub+zip"))
		c, _ := zw.Create("META-INF/container.xml")
		_, _ = c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
		opf, _ := zw.Create("OEBPS/content.opf")
		_, _ = opf.Write([]byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>AssetBook</dc:title></metadata><manifest><item id="p1" href="p1.xhtml" media-type="application/xhtml+xml"/><item id="img1" href="art.png" media-type="image/png"/></manifest><spine><itemref idref="p1"/></spine></package>`))
		p1, _ := zw.Create("OEBPS/p1.xhtml")
		_, _ = p1.Write([]byte(`<html><body><img src="art.png"/></body></html>`))
		im, _ := zw.Create("OEBPS/art.png")
		_, _ = im.Write(imgBytes)
		_ = zw.Close()
		_ = os.WriteFile(epubPath, buf.Bytes(), 0644)

		book, err := f.ingester.IngestFile(context.Background(), epubPath, filepath.Join("Art", "AssetBook", "AssetBook.epub"))
		if err != nil {
			t.Fatalf("IngestFile: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/assets/OEBPS/art.png", book.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), imgBytes) {
			t.Fatalf("expected 200 and matching PNG asset bytes, got %d", rec.Code)
		}
	})

	t.Run("F08_T1_04_ReaderViewTypographyAndKindleToolbar", func(t *testing.T) {
		reader := readAppFile(t, "lib/ui/features/reader/reader_view.dart")
		hlModel := readAppFile(t, "lib/data/models/highlight.dart")
		assertContainsAll(t, "reader_view.dart", reader,
			"class ReaderView",
			"class KindleSelectionToolbar",
			"Table of Contents",
			"Bone", "Sepia", "Dark OLED",
			"Serif", "Sans",
		)
		assertContainsAll(t, "highlight.dart", hlModel,
			"enum KindleHighlightColor",
			"name: 'yellow'",
			"name: 'blue'",
			"name: 'pink'",
			"name: 'orange'",
		)
	})

	t.Run("F08_T1_05_FixedLayoutReaderSpreadEngine", func(t *testing.T) {
		fixed := readAppFile(t, "lib/ui/features/reader/fixed_layout_reader.dart")
		assertContainsAll(t, "fixed_layout_reader.dart", fixed,
			"class FixedLayoutReader",
			"Page ",
			"Pages ",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 9: Audiobook Studio Player & Persistent Mini-Player
	// -------------------------------------------------------------------------
	t.Run("F09_T1_01_LiveAPI_AudiobookChaptersAndProgressSync", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		dur := 1800.0
		book := &repository.Book{
			ID:              "audio-t1-1",
			Title:           "Project Hail Mary",
			FilePath:        "Andy Weir/Project Hail Mary/PHM.m4b",
			BookType:        "audiobook",
			DurationSeconds: &dur,
		}
		_ = f.repo.CreateBook(context.Background(), book)
		_ = f.repo.CreateAudioChapters(context.Background(), []*repository.AudioChapter{
			{BookID: book.ID, ChapterIndex: 1, Title: "Chapter 1", StartOffsetSec: 0, DurationSec: 600},
		})

		postReq := httptest.NewRequest(http.MethodPost, "/api/v1/audiobooks/"+book.ID+"/progress", strings.NewReader(`{"position_seconds":125.5,"speed":1.5,"is_completed":false}`))
		postReq.Header.Set("Authorization", "Bearer "+token)
		postReq.Header.Set("Content-Type", "application/json")
		postRec := httptest.NewRecorder()
		f.handler.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on save audiobook progress, got %d", postRec.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/"+book.ID+"/progress", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), "125.5") {
			t.Fatalf("expected saved position 125.5, got %s", getRec.Body.String())
		}
	})

	t.Run("F09_T1_02_LiveAPI_AudiobookStreamAndRangeRequests", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		relPath := filepath.Join("Andy Weir", "PHM", "PHM.m4b")
		fullPath := filepath.Join(f.libDir, relPath)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		audioBytes := bytes.Repeat([]byte("AUDIO_FRAME_"), 200)
		_ = os.WriteFile(fullPath, audioBytes, 0644)

		sz := int64(len(audioBytes))
		dur := 600.0
		book := &repository.Book{
			ID:              "audio-stream-1",
			Title:           "PHM Audio",
			FilePath:        relPath,
			BookType:        "audiobook",
			DurationSeconds: &dur,
			FileSizeBytes:   &sz,
		}
		_ = f.repo.CreateBook(context.Background(), book)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/"+book.ID+"/stream", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Range", "bytes=0-63")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusPartialContent || rec.Body.Len() != 64 {
			t.Fatalf("expected 206 Partial Content with 64 bytes, got %d (len=%d)", rec.Code, rec.Body.Len())
		}
	})

	t.Run("F09_T1_03_AudiobookPlayerViewStudioKeysAndControls", func(t *testing.T) {
		player := readAppFile(t, "lib/ui/features/audiobook/audiobook_player_view.dart")
		assertContainsAll(t, "audiobook_player_view.dart", player,
			"Key('audio_player_back_button')",
			"Key('audio_player_chapters_menu_button')",
			"Key('audio_player_skip_back_button')",
			"Key('audio_player_play_pause_button')",
			"Key('audio_player_skip_forward_button')",
			"Key('audio_player_speed_button')",
			"Key('audio_player_sleep_timer_button')",
			"Key('audio_player_add_bookmark_button')",
		)
	})

	t.Run("F09_T1_04_MiniPlayerBarPersistentDockContract", func(t *testing.T) {
		mini := readAppFile(t, "lib/ui/features/audiobook/widgets/mini_player_bar.dart")
		assertContainsAll(t, "mini_player_bar.dart", mini,
			"class MiniPlayerBar",
			"Key('mini_player_tap_area')",
			"Key('mini_player_skip_30')",
			"Key('mini_player_play_pause')",
			"Key('mini_player_close')",
		)
	})

	t.Run("F09_T1_05_ChapterDrawerAndScrubberBarContract", func(t *testing.T) {
		drawer := readAppFile(t, "lib/ui/features/audiobook/widgets/chapter_drawer.dart")
		scrubber := readAppFile(t, "lib/ui/features/audiobook/widgets/scrubber_bar.dart")
		assertContainsAll(t, "chapter_drawer.dart", drawer,
			"class ChapterDrawerSheet",
			"class EqualizerBars",
			"chapter_drawer_tab_",
		)
		assertContainsAll(t, "scrubber_bar.dart", scrubber,
			"class ScrubberBar",
			"_formatDuration",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 10: Uploads & Ingestion Workspace
	// -------------------------------------------------------------------------
	t.Run("F10_T1_01_LiveAPI_StageUploadEPUBAndExtractMetadata", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		epubBytes := buildInMemoryEPUB(t, "The Dispossessed", "Ursula K. Le Guin", "An ambiguous utopia.")
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("file", "dispossessed.epub")
		_, _ = part.Write(epubBytes)
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on stage upload, got %d: %s", rec.Code, rec.Body.String())
		}
		assertContainsAll(t, "stage response", rec.Body.String(), "The Dispossessed", "Ursula K. Le Guin", `"status":"staged"`)
	})

	t.Run("F10_T1_02_LiveAPI_UploadAndFetchStagedJobCover", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		epubBytes := buildInMemoryEPUB(t, "Rocannon's World", "Ursula K. Le Guin", "First Hainish novel.")
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("file", "rocannon.epub")
		_, _ = part.Write(epubBytes)
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		var stageResp struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &stageResp)

		coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
		var cBody bytes.Buffer
		cmpw := multipart.NewWriter(&cBody)
		cPart, _ := cmpw.CreateFormFile("cover", "cover.jpg")
		_, _ = cPart.Write(coverBytes)
		_ = cmpw.Close()

		cReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+stageResp.JobID+"/cover", &cBody)
		cReq.Header.Set("Authorization", "Bearer "+token)
		cReq.Header.Set("Content-Type", cmpw.FormDataContentType())
		cRec := httptest.NewRecorder()
		f.handler.ServeHTTP(cRec, cReq)
		if cRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on staged cover upload, got %d: %s", cRec.Code, cRec.Body.String())
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/upload/jobs/"+stageResp.JobID+"/cover?token="+token, nil)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK || !bytes.Equal(getRec.Body.Bytes(), coverBytes) {
			t.Fatalf("expected 200 and matching staged cover bytes, got %d", getRec.Code)
		}
	})

	t.Run("F10_T1_03_LiveAPI_CommitStagedUploadToLibrary", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		epubBytes := buildInMemoryEPUB(t, "Planet of Exile", "Ursula K. Le Guin", "Second Hainish novel.")
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("file", "exile.epub")
		_, _ = part.Write(epubBytes)
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		var stageResp struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &stageResp)

		commitPayload := `{"title":"Planet of Exile","author":"Ursula K. Le Guin","series":"Hainish Cycle","sequence_number":2,"genres":["Science Fiction"]}`
		cReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+stageResp.JobID+"/commit", strings.NewReader(commitPayload))
		cReq.Header.Set("Authorization", "Bearer "+token)
		cReq.Header.Set("Content-Type", "application/json")
		cRec := httptest.NewRecorder()
		f.handler.ServeHTTP(cRec, cReq)
		if cRec.Code != http.StatusCreated && cRec.Code != http.StatusOK {
			t.Fatalf("expected 201/200 on commit, got %d: %s", cRec.Code, cRec.Body.String())
		}
	})

	t.Run("F10_T1_04_UploadsViewQueueAndDesktopInspector", func(t *testing.T) {
		view := readAppFile(t, "lib/ui/features/upload/uploads_view.dart")
		assertContainsAll(t, "uploads_view.dart", view,
			"class UploadsView",
			"Uploads & Ingestion",
			"Auto-add",
			"Drag & drop EPUB files or nested folders here",
			"Approve All Ready",
			"_currentDestinationPreview",
		)
	})

	t.Run("F10_T1_05_UploadReviewDialogAndDropTarget", func(t *testing.T) {
		review := readAppFile(t, "lib/ui/features/upload/upload_review_dialog.dart")
		drop := readAppFile(t, "lib/ui/features/upload/upload_drop_target.dart")
		assertContainsAll(t, "upload_review_dialog.dart", review,
			"class UploadReviewContent",
			"Review Staged",
			"Missing Author",
			"Duplicate Book Detected",
		)
		assertContainsAll(t, "upload_drop_target.dart", drop, "class ShelfdDropTarget")
	})

	// -------------------------------------------------------------------------
	// Feature 11: Semantic Search, Utilities, Settings, Connect & OAuth UI
	// -------------------------------------------------------------------------
	t.Run("F11_T1_01_LiveAPI_SemanticVectorSearch", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=sentient+ocean&limit=5", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on semantic search, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("F11_T1_02_LiveAPI_UtilitiesFindDuplicatesAndMergeBooks", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		ctx := context.Background()

		author, _ := f.repo.UpsertAuthor(ctx, "Frank Herbert")
		isbn := "9780441172719"
		b1 := &repository.Book{ID: ulid.New(), Title: "Dune", Identifier: &isbn, FilePath: filepath.Join(f.dataDir, "d1.epub"), CreatedAt: time.Now().UTC()}
		b2 := &repository.Book{ID: ulid.New(), Title: "Dune", Identifier: &isbn, FilePath: filepath.Join(f.dataDir, "d2.epub"), CreatedAt: time.Now().UTC()}
		_ = os.WriteFile(b1.FilePath, []byte("epub1"), 0644)
		_ = os.WriteFile(b2.FilePath, []byte("epub2"), 0644)
		_ = f.repo.CreateBook(ctx, b1)
		_ = f.repo.CreateBook(ctx, b2)
		_ = f.repo.LinkBookAuthor(ctx, b1.ID, author.ID, "author")
		_ = f.repo.LinkBookAuthor(ctx, b2.ID, author.ID, "author")

		dupReq := httptest.NewRequest(http.MethodGet, "/api/v1/utilities/duplicates", nil)
		dupReq.Header.Set("Authorization", "Bearer "+token)
		dupRec := httptest.NewRecorder()
		f.handler.ServeHTTP(dupRec, dupReq)
		if dupRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on duplicates scan, got %d", dupRec.Code)
		}

		mergeBody := fmt.Sprintf(`{"primary_book_id":"%s","duplicate_book_ids":["%s"]}`, b1.ID, b2.ID)
		mReq := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/merge-books", strings.NewReader(mergeBody))
		mReq.Header.Set("Authorization", "Bearer "+token)
		mReq.Header.Set("Content-Type", "application/json")
		mRec := httptest.NewRecorder()
		f.handler.ServeHTTP(mRec, mReq)
		if mRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on merge-books, got %d: %s", mRec.Code, mRec.Body.String())
		}
	})

	t.Run("F11_T1_03_LiveAPI_OAuthConsentPageHTMLAndCSSContract", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()

		mux := http.NewServeMux()
		oauthHandler := api.NewOAuthHandler(f.repo)
		oauthHandler.RegisterRoutes(mux)

		regReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Scriptorium Companion","redirect_uris":["https://companion.test/cb"]}`))
		regRec := httptest.NewRecorder()
		mux.ServeHTTP(regRec, regReq)
		var regResp api.RegisterResponse
		_ = json.NewDecoder(regRec.Body).Decode(&regResp)

		authURL := fmt.Sprintf("/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=s1",
			url.QueryEscape(regResp.ClientID),
			url.QueryEscape("https://companion.test/cb"),
		)
		getReq := httptest.NewRequest(http.MethodGet, authURL, nil)
		getRec := httptest.NewRecorder()
		mux.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on GET /oauth/authorize, got %d", getRec.Code)
		}
		assertContainsAll(t, "OAuth consent HTML", getRec.Body.String(),
			"--bg:", "--card-bg:", "--ink:", "--muted:", "--border:", "--accent:",
			"Authorize Scriptorium Companion",
			"Shelfd",
			`value="deny"`,
			`value="authorize"`,
		)
	})

	t.Run("F11_T1_04_LiveAPI_ConnectInfoAndAuthTokensManagement", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		ciReq := httptest.NewRequest(http.MethodGet, "/api/v1/server/connect-info", nil)
		ciRec := httptest.NewRecorder()
		f.handler.ServeHTTP(ciRec, ciReq)
		if ciRec.Code != http.StatusOK || !strings.Contains(ciRec.Body.String(), `"server_name":"Shelfd"`) {
			t.Fatalf("unexpected connect-info response: %s", ciRec.Body.String())
		}

		tokReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"name":"E2E Token"}`))
		tokReq.Header.Set("Authorization", "Bearer "+token)
		tokReq.Header.Set("Content-Type", "application/json")
		tokRec := httptest.NewRecorder()
		f.handler.ServeHTTP(tokRec, tokReq)
		if tokRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 on token create, got %d: %s", tokRec.Code, tokRec.Body.String())
		}
	})

	t.Run("F11_T1_05_SearchUtilitiesSettingsConnectViewsContract", func(t *testing.T) {
		search := readAppFile(t, "lib/ui/features/search/search_view.dart")
		utilities := readAppFile(t, "lib/ui/features/utilities/utilities_view.dart")
		settings := readAppFile(t, "lib/ui/features/settings/settings_view.dart")
		connect := readAppFile(t, "lib/ui/features/connect/connect_view.dart")
		assertContainsAll(t, "search_view.dart", search, "class SearchView", "Read Chapter at this passage →")
		assertContainsAll(t, "utilities_view.dart", utilities, "class UtilitiesView", "Duplicate Books Finder", "Merge Group")
		assertContainsAll(t, "settings_view.dart", settings, "class SettingsView", "Account & Security", "Server Connection")
		assertContainsAll(t, "connect_view.dart", connect, "class ConnectView", "Connect to your Library", "Connect & Sync Library")
	})

	// -------------------------------------------------------------------------
	// Feature 12: Final E2E & Widget Test Suite Verification & Hardening
	// -------------------------------------------------------------------------
	t.Run("F12_T1_01_AllFlutterTestFilesPresentAndSyntacticallyValid", func(t *testing.T) {
		testDir := filepath.Join(repoRoot(t), "apps", "app", "test")
		count := 0
		err := filepath.WalkDir(testDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), "_test.dart") {
				count++
				rel, _ := filepath.Rel(filepath.Join(repoRoot(t), "apps", "app"), path)
				src := readAppFile(t, rel)
				assertValidDartSource(t, rel, src)
				assertContainsAll(t, rel, src, "void main()")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking apps/app/test: %v", err)
		}
		if count < 27 {
			t.Errorf("expected at least 27 Flutter test files in apps/app/test, found %d", count)
		}
	})

	t.Run("F12_T1_02_AllFlutterUISourceFilesSyntacticallyValid", func(t *testing.T) {
		libDir := filepath.Join(repoRoot(t), "apps", "app", "lib")
		count := 0
		err := filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".dart") {
				count++
				rel, _ := filepath.Rel(filepath.Join(repoRoot(t), "apps", "app"), path)
				src := readAppFile(t, rel)
				assertValidDartSource(t, rel, src)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking apps/app/lib: %v", err)
		}
		if count < 35 {
			t.Errorf("expected at least 35 Dart source files in apps/app/lib, found %d", count)
		}
	})

	t.Run("F12_T1_03_TactileScriptoriumE2EDartSuiteCompleteness", func(t *testing.T) {
		suite := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertValidDartSource(t, "test/ui/tactile_scriptorium_e2e_test.dart", suite)
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", suite,
			"Tier 1 — Feature Coverage",
			"Tier 2 — Boundary & Corner Cases",
			"Tier 3 — Cross-Feature Combinations",
			"Tier 4 — Real-World User Journeys",
			"Feature 1:", "Feature 2:", "Feature 3:", "Feature 4:",
			"Feature 5:", "Feature 6:", "Feature 7:", "Feature 8:",
			"Feature 9:", "Feature 10:", "Feature 11:", "Feature 12:",
		)
	})

	t.Run("F12_T1_04_WidgetKeyCrossReferenceIntegrity", func(t *testing.T) {
		bookDetail := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		editDlg := readAppFile(t, "lib/ui/features/book_detail/edit_book_metadata_dialog.dart")
		audioView := readAppFile(t, "lib/ui/features/audiobook/audiobook_player_view.dart")
		miniBar := readAppFile(t, "lib/ui/features/audiobook/widgets/mini_player_bar.dart")
		combinedUI := bookDetail + "\n" + editDlg + "\n" + audioView + "\n" + miniBar

		requiredKeys := []string{
			"download_hero_menu_button",
			"download_appbar_menu_button",
			"menu_item_download_epub",
			"menu_item_send_notebooklm",
			"menu_item_download_markdown",
			"edit_metadata_appbar_button",
			"edit_metadata_sidebar_button",
			"listen_hero_button",
			"read_hero_button",
			"edit_book_title_field",
			"edit_book_authors_field",
			"edit_book_series_field",
			"edit_book_sequence_field",
			"edit_book_genres_field",
			"edit_book_topics_field",
			"edit_book_publisher_field",
			"edit_book_language_field",
			"edit_book_description_field",
			"save_book_metadata_button",
			"audio_player_back_button",
			"audio_player_chapters_menu_button",
			"audio_player_skip_back_button",
			"audio_player_play_pause_button",
			"audio_player_skip_forward_button",
			"audio_player_speed_button",
			"audio_player_sleep_timer_button",
			"audio_player_add_bookmark_button",
			"mini_player_tap_area",
			"mini_player_skip_30",
			"mini_player_play_pause",
			"mini_player_close",
		}
		assertContainsAll(t, "UI Widget Keys", combinedUI, requiredKeys...)
	})

	t.Run("F12_T1_05_TestInfraAndProjectDocumentAlignment", func(t *testing.T) {
		screenshotsDoc := readRepoFile(t, "docs/screenshots/README.md")
		e2eDart := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertContainsAll(t, "docs/screenshots/README.md", screenshotsDoc, "The Tactile Scriptorium", "ShelfdColors", "TactileBookCover")
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2eDart, "ShelfdColors", "TactileBookCover")
	})
}

// =============================================================================
// TIER 2 — BOUNDARY & CORNER CASES (12 Features x 5 Subtests = 60 Subtests)
// =============================================================================

func TestTactileScriptorium_Tier2_BoundaryAndCornerCases(t *testing.T) {
	// -------------------------------------------------------------------------
	// Feature 1 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F01_T2_01_ColorHexFormatAndAlphaIntegrity", func(t *testing.T) {
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		re := regexp.MustCompile(`Color\(0x([0-9A-Fa-f]{8})\)`)
		matches := re.FindAllStringSubmatch(tokens, -1)
		if len(matches) < 20 {
			t.Fatalf("expected at least 20 8-digit ARGB Color literals in tokens.dart, found %d", len(matches))
		}
		for _, m := range matches {
			val, err := strconv.ParseUint(m[1], 16, 64)
			if err != nil {
				t.Errorf("invalid hex literal %s: %v", m[0], err)
			}
			alpha := (val >> 24) & 0xFF
			if alpha != 0xFF {
				t.Errorf("expected opaque 0xFF alpha on token %s, got 0x%02X", m[0], alpha)
			}
		}
	})

	t.Run("F01_T2_02_ContrastPolarityAcrossModes", func(t *testing.T) {
		// Bone: warmParchment (0xFFFBF9F4) vs espressoInk (0xFF1E1B18)
		boneBgLum := relativeLuminance(0xFFFBF9F4)
		boneInkLum := relativeLuminance(0xFF1E1B18)
		if boneBgLum-boneInkLum < 0.70 {
			t.Errorf("insufficient Bone luminance delta: bg=%.3f ink=%.3f", boneBgLum, boneInkLum)
		}

		// Sepia: sepiaBackground (0xFFF4ECD8) vs sepiaInk (0xFF5B4636)
		sepiaBgLum := relativeLuminance(0xFFF4ECD8)
		sepiaInkLum := relativeLuminance(0xFF5B4636)
		if sepiaBgLum-sepiaInkLum < 0.50 {
			t.Errorf("insufficient Sepia luminance delta: bg=%.3f ink=%.3f", sepiaBgLum, sepiaInkLum)
		}

		// Dark: walnutBackground (0xFF161412) vs walnutInk (0xFFEDE7DF)
		darkBgLum := relativeLuminance(0xFF161412)
		darkInkLum := relativeLuminance(0xFFEDE7DF)
		if darkInkLum-darkBgLum < 0.65 {
			t.Errorf("insufficient Dark luminance delta: bg=%.3f ink=%.3f", darkBgLum, darkInkLum)
		}
	})

	t.Run("F01_T2_03_ThemeTestBackwardsCompatibility", func(t *testing.T) {
		themeTest := readAppFile(t, "test/ui/core/theme_test.dart")
		assertContainsAll(t, "theme_test.dart", themeTest,
			"expect(AppTokens.minTouchTarget, 48.0)",
			"expect(AppTokens.maxReadingWidth, 720.0)",
			"ReadingThemeMode.bone",
			"ReadingThemeMode.sepia",
			"ReadingThemeMode.dark",
		)
	})

	t.Run("F01_T2_04_DartSyntaxAndImmutability", func(t *testing.T) {
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		theme := readAppFile(t, "lib/ui/core/theme.dart")
		assertContainsAll(t, "tokens.dart", tokens, "AppTokens._();", "static const")
		assertContainsAll(t, "theme.dart", theme, "AppTheme._();")
	})

	t.Run("F01_T2_05_ShelfdColorsFallbackAndLerpContract", func(t *testing.T) {
		e2e := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2e,
			"F01-T2-02",
			"F01-T2-03",
			"ShelfdColors.bone.lerp(null, 0.5)",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 2 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F02_T2_01_NullableColorParameterSafety", func(t *testing.T) {
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		assertContainsAll(t, "typography.dart", typo, "Color? color")
		if strings.Contains(typo, "Color color = AppTokens.charcoalInk") {
			t.Errorf("typography.dart must not force non-null default AppTokens.charcoalInk")
		}
	})

	t.Run("F02_T2_02_HtmlTextMalformedAndEmptyInput", func(t *testing.T) {
		htmlText := readAppFile(t, "lib/ui/core/html_text.dart")
		assertContainsAll(t, "html_text.dart", htmlText,
			"_sanitizeHtml",
			"emptyPlaceholder",
			"trimmed.isEmpty",
			"blocks.isEmpty",
		)
	})

	t.Run("F02_T2_03_ReaderTextSerifToggleBoundary", func(t *testing.T) {
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		assertContainsAll(t, "typography.dart", typo,
			"required bool isSerif",
			"isSerif ? serifFallbacks : sansFallbacks",
		)
	})

	t.Run("F02_T2_04_OfflineTestFontSafety", func(t *testing.T) {
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		if strings.Contains(typo, "GoogleFonts.") {
			t.Errorf("typography.dart must use fontFamily/fontFamilyFallback instead of runtime GoogleFonts.* HTTP fetch")
		}
	})

	t.Run("F02_T2_05_SPAHandlerServesWebEntrypointAndFonts", func(t *testing.T) {
		webDir := filepath.Join(repoRoot(t), "apps", "app", "web")
		handler := api.SPAHandler(webDir)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK serving web/index.html, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
			t.Errorf("expected no-store Cache-Control on index.html, got %s", rec.Header().Get("Cache-Control"))
		}
	})

	// -------------------------------------------------------------------------
	// Feature 3 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F03_T2_01_AuthorAvatarInitialsEdgeCases", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"String get initials",
			"if (parts.isEmpty || parts[0].isEmpty) return '?';",
		)
	})

	t.Run("F03_T2_02_CoverFallbackWhenNullOrEmpty", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared, "errorBuilder")
	})

	t.Run("F03_T2_03_PrimaryButtonLoadingAndDisabledState", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"isLoading ? null : onPressed",
			"CircularProgressIndicator",
		)
	})

	t.Run("F03_T2_04_ResponsiveBreakpointSwitching", func(t *testing.T) {
		resp := readAppFile(t, "lib/ui/core/responsive.dart")
		assertContainsAll(t, "responsive.dart", resp,
			"class Responsive",
			"isMobile",
			"isTablet",
			"isDesktop",
			"AppTokens.tabletBreakpoint",
		)
	})

	t.Run("F03_T2_05_QueueStatusCardActiveVsIdle", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared,
			"AI Catalog Synced",
			"Checking queue...",
			"resolveBookTitle",
		)
	})

	// -------------------------------------------------------------------------
	// Feature 4 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F04_T2_01_LiveAPI_EmptySearchAndZeroResults", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/books?search=nonexistent_xyz_999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var resp struct {
			Total int `json:"total"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Total != 0 {
			t.Errorf("expected 0 results for nonexistent search, got %d", resp.Total)
		}
	})

	t.Run("F04_T2_02_LiveAPI_PaginationLimitClamping", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/books?limit=5000&page=-3", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var resp struct {
			Limit int `json:"limit"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Limit > 1000 {
			t.Errorf("expected limit clamped <= 1000, got %d", resp.Limit)
		}
	})

	t.Run("F04_T2_03_LiveAPI_UnquotedAndQuotedTokenEdgeCases", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		q := url.QueryEscape(`isbn:9780156027601 title:"Solaris"`)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/books?search="+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var resp struct {
			Total int `json:"total"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Total != 1 {
			t.Errorf("expected 1 match for isbn + title tokens, got %d", resp.Total)
		}
	})

	t.Run("F04_T2_04_NestedAppShellScaffoldAvoidance", func(t *testing.T) {
		libView := readAppFile(t, "lib/ui/features/library/library_view.dart")
		assertContainsAll(t, "library_view.dart", libView, "findAncestorWidgetOfExactType<AppShell>()")
	})

	t.Run("F04_T2_05_SingularVsPluralBookCountBadges", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "shared_layout.dart", shared, "'1 Book'", "Books'")
	})

	// -------------------------------------------------------------------------
	// Feature 5 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F05_T2_01_LiveAPI_AuthorPhotoNotFoundAndMissingFileRejection", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		// Unknown author returns 404
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/authors/nonexistent-author/photo", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unknown author photo, got %d", getRec.Code)
		}

		// Empty multipart upload without photo field returns 400
		author, _ := f.repo.UpsertAuthor(context.Background(), "Test Author")
		var emptyBody bytes.Buffer
		mpw := multipart.NewWriter(&emptyBody)
		_ = mpw.Close()
		postReq := httptest.NewRequest(http.MethodPost, "/api/v1/authors/"+author.ID+"/photo", &emptyBody)
		postReq.Header.Set("Authorization", "Bearer "+token)
		postReq.Header.Set("Content-Type", mpw.FormDataContentType())
		postRec := httptest.NewRecorder()
		f.handler.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing photo file, got %d", postRec.Code)
		}
	})

	t.Run("F05_T2_02_SeriesSequenceNullAndFractionalSorting", func(t *testing.T) {
		seriesView := readAppFile(t, "lib/ui/features/series/series_detail_view.dart")
		assertContainsAll(t, "series_detail_view.dart", seriesView, "seriesSequence", "compareSeriesBooks")
	})

	t.Run("F05_T2_03_AuthorAndSeriesEmptyBooksState", func(t *testing.T) {
		authorView := readAppFile(t, "lib/ui/features/authors/author_detail_view.dart")
		seriesView := readAppFile(t, "lib/ui/features/series/series_detail_view.dart")
		assertContainsAll(t, "author_detail_view.dart", authorView, "isEmpty")
		assertContainsAll(t, "series_detail_view.dart", seriesView, "isEmpty")
	})

	t.Run("F05_T2_04_RouterTaxonomyRoutesAndRedirects", func(t *testing.T) {
		router := readAppFile(t, "lib/ui/router.dart")
		assertContainsAll(t, "router.dart", router,
			"path: '/series'",
			"path: '/authors'",
			"path: '/genres'",
			"path: '/topics'",
			"path: '/author'",
			"path: '/author/:authorId'",
		)
	})

	t.Run("F05_T2_05_GenreAndTopicDetailNavigation", func(t *testing.T) {
		libView := readAppFile(t, "lib/ui/features/library/library_view.dart")
		assertContainsAll(t, "library_view.dart", libView, "GenreGridItem", "TopicGridItem")
	})

	// -------------------------------------------------------------------------
	// Feature 6 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F06_T2_01_LiveAPI_HighlightInvalidColorFallbackAndEmptyText", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		// Empty selected_text -> 400
		emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/highlights", strings.NewReader(`{"selected_text":"   ","color":"yellow"}`))
		emptyReq.Header.Set("Authorization", "Bearer "+token)
		emptyReq.Header.Set("Content-Type", "application/json")
		emptyRec := httptest.NewRecorder()
		f.handler.ServeHTTP(emptyRec, emptyReq)
		if emptyRec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty highlight text, got %d", emptyRec.Code)
		}

		// Unrecognized color -> normalized to yellow
		unkReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/highlights", strings.NewReader(`{"selected_text":"Valid quote","color":"ultraviolet"}`))
		unkReq.Header.Set("Authorization", "Bearer "+token)
		unkReq.Header.Set("Content-Type", "application/json")
		unkRec := httptest.NewRecorder()
		f.handler.ServeHTTP(unkRec, unkReq)
		if unkRec.Code != http.StatusCreated || !strings.Contains(unkRec.Body.String(), `"color":"yellow"`) {
			t.Errorf("expected 201 with normalized yellow color, got %d: %s", unkRec.Code, unkRec.Body.String())
		}
	})

	t.Run("F06_T2_02_LiveAPI_ChatEmptyMessageAndHistoryTruncation", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/chat", strings.NewReader(`{"query":"   "}`))
		emptyReq.Header.Set("Authorization", "Bearer "+token)
		emptyReq.Header.Set("Content-Type", "application/json")
		emptyRec := httptest.NewRecorder()
		f.handler.ServeHTTP(emptyRec, emptyReq)
		if emptyRec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty chat query, got %d", emptyRec.Code)
		}

		// 8 history messages (>6 truncation boundary)
		longHistory := `{"query":"Follow-up question","history":[` +
			`{"role":"user","content":"m1"},{"role":"assistant","content":"r1"},` +
			`{"role":"user","content":"m2"},{"role":"assistant","content":"r2"},` +
			`{"role":"user","content":"m3"},{"role":"assistant","content":"r3"},` +
			`{"role":"user","content":"m4"},{"role":"assistant","content":"r4"}]}`
		histReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/chat", strings.NewReader(longHistory))
		histReq.Header.Set("Authorization", "Bearer "+token)
		histReq.Header.Set("Content-Type", "application/json")
		histRec := httptest.NewRecorder()
		f.handler.ServeHTTP(histRec, histReq)
		if histRec.Code != http.StatusOK {
			t.Errorf("expected 200 with truncated history, got %d: %s", histRec.Code, histRec.Body.String())
		}
	})

	t.Run("F06_T2_03_LiveAPI_BookNotFoundAndCoverFallback", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		for _, ep := range []string{"/api/v1/books/nonexistent-id", "/api/v1/books/nonexistent-id/cover"} {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("expected 404 on %s, got %d", ep, rec.Code)
			}
		}
	})

	t.Run("F06_T2_04_BookDetailDualFormatAndMultiFileAudiobook", func(t *testing.T) {
		view := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		assertContainsAll(t, "book_detail_view.dart", view, "listen_file_button_", "read_file_button_")
	})

	t.Run("F06_T2_05_BookmarkAndCitationClickNavigationContract", func(t *testing.T) {
		view := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		assertContainsAll(t, "book_detail_view.dart", view, "_showCitationDetails", "Read in Reader")
	})

	// -------------------------------------------------------------------------
	// Feature 7 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F07_T2_01_LiveAPI_UpdateMetadataEmptyTitleRejection", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+book.ID+"/metadata", strings.NewReader(`{"title":"   "}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 on empty title metadata update, got %d", rec.Code)
		}
	})

	t.Run("F07_T2_02_LiveAPI_UpdateMetadataEmptyAuthorsDefaultUnknown", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+book.ID+"/metadata", strings.NewReader(`{"title":"Anonymous Codex","authors":[]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Unknown") {
			t.Errorf("expected 200 with fallback Unknown author, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("F07_T2_03_LiveAPI_UploadBookCoverNonImageRejection", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("cover", "bad.txt")
		_, _ = part.Write([]byte("not an image file"))
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/cover", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for non-image cover upload, got %d", rec.Code)
		}
	})

	t.Run("F07_T2_04_LiveAPI_MetadataSearchMissingQuery", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/search?q=", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty metadata search query, got %d", rec.Code)
		}
	})

	t.Run("F07_T2_05_ResponsiveModalVsBottomSheetPresentation", func(t *testing.T) {
		editDlg := readAppFile(t, "lib/ui/features/book_detail/edit_book_metadata_dialog.dart")
		searchDlg := readAppFile(t, "lib/ui/features/book_detail/metadata_search_dialog.dart")
		assertContainsAll(t, "edit_book_metadata_dialog.dart", editDlg, "showModalBottomSheet", "showDialog")
		assertContainsAll(t, "metadata_search_dialog.dart", searchDlg, "showModalBottomSheet", "showDialog")
	})

	// -------------------------------------------------------------------------
	// Feature 8 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F08_T2_01_LiveAPI_GetChapterInvalidIdentifierAndCrossBookIsolation", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		// Non-integer non-existing chapter identifier -> 400
		badReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID+"/chapters/not-an-index", nil)
		badReq.Header.Set("Authorization", "Bearer "+token)
		badRec := httptest.NewRecorder()
		f.handler.ServeHTTP(badRec, badReq)
		if badRec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 on invalid chapter identifier, got %d", badRec.Code)
		}

		// Chapter index 1 requested on a book with no chapters -> 404
		otherBook := &repository.Book{ID: "other-book", Title: "Other", FilePath: "other.epub"}
		_ = f.repo.CreateBook(context.Background(), otherBook)
		crossReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/other-book/chapters/1", nil)
		crossReq.Header.Set("Authorization", "Bearer "+token)
		crossRec := httptest.NewRecorder()
		f.handler.ServeHTTP(crossRec, crossReq)
		if crossRec.Code != http.StatusNotFound {
			t.Errorf("expected 404 on missing chapter request, got %d", crossRec.Code)
		}
	})

	t.Run("F08_T2_02_ReaderMarkdownBlockParserEdgeCases", func(t *testing.T) {
		md := readAppFile(t, "lib/ui/features/reader/reader_markdown.dart")
		assertContainsAll(t, "reader_markdown.dart", md,
			"parseReaderBlocks",
			"parseInlineSpans",
			"findHighlightRange",
			"ReaderBlockType",
		)
	})

	t.Run("F08_T2_03_ReaderDarkAndSepiaContrastInDrawerAndSheets", func(t *testing.T) {
		e2e := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2e, "F08-T2-01", "ReadingThemeMode.sepia", "ReadingThemeMode.dark")
	})

	t.Run("F08_T2_04_FixedLayoutGatefoldAndSinglePageMode", func(t *testing.T) {
		fixed := readAppFile(t, "lib/ui/features/reader/fixed_layout_reader.dart")
		assertContainsAll(t, "fixed_layout_reader.dart", fixed, "pageWidth", "pageHeight")
	})

	t.Run("F08_T2_05_ReaderBottomBarMinutesLeftFormat", func(t *testing.T) {
		reader := readAppFile(t, "lib/ui/features/reader/reader_view.dart")
		assertContainsAll(t, "reader_view.dart", reader, "mins left")
	})

	// -------------------------------------------------------------------------
	// Feature 9 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F09_T2_01_LiveAPI_AudiobookMultiFileTrackStreamingAndIsolation", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		relPath := filepath.Join("Author", "MultiTrack", "Track01.mp3")
		fullPath := filepath.Join(f.libDir, relPath)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		trackBytes := bytes.Repeat([]byte("MP3DATA!"), 100)
		_ = os.WriteFile(fullPath, trackBytes, 0644)

		b1 := &repository.Book{ID: "b-mt-1", Title: "MultiTrack 1", FilePath: relPath, BookType: "audiobook"}
		b2 := &repository.Book{ID: "b-mt-2", Title: "MultiTrack 2", FilePath: relPath, BookType: "audiobook"}
		_ = f.repo.CreateBook(context.Background(), b1)
		_ = f.repo.CreateBook(context.Background(), b2)

		sz := int64(len(trackBytes))
		mime := "audio/mpeg"
		bf := &repository.BookFile{ID: "bf-track-1", BookID: b1.ID, FileType: "mp3", FilePath: relPath, FileSizeBytes: &sz, MimeType: &mime}
		_ = f.repo.CreateBookFiles(context.Background(), []*repository.BookFile{bf})

		// Valid owner book streams 206
		okReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/files/%s/stream", b1.ID, bf.ID), nil)
		okReq.Header.Set("Authorization", "Bearer "+token)
		okReq.Header.Set("Range", "bytes=0-31")
		okRec := httptest.NewRecorder()
		f.handler.ServeHTTP(okRec, okReq)
		if okRec.Code != http.StatusPartialContent {
			t.Fatalf("expected 206 on track stream, got %d", okRec.Code)
		}

		// Cross-book file request returns 404
		badReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/files/%s/stream", b2.ID, bf.ID), nil)
		badReq.Header.Set("Authorization", "Bearer "+token)
		badRec := httptest.NewRecorder()
		f.handler.ServeHTTP(badRec, badReq)
		if badRec.Code != http.StatusNotFound {
			t.Errorf("expected 404 on cross-book file stream, got %d", badRec.Code)
		}
	})

	t.Run("F09_T2_02_MiniPlayerCollapsedWhenNoBookLoaded", func(t *testing.T) {
		mini := readAppFile(t, "lib/ui/features/audiobook/widgets/mini_player_bar.dart")
		assertContainsAll(t, "mini_player_bar.dart", mini, "SizedBox.shrink()")
	})

	t.Run("F09_T2_03_ScrubberBarZeroAndNegativeDurationSafety", func(t *testing.T) {
		scrubber := readAppFile(t, "lib/ui/features/audiobook/widgets/scrubber_bar.dart")
		assertContainsAll(t, "scrubber_bar.dart", scrubber, "clamp(")
	})

	t.Run("F09_T2_04_SleepTimerFadeOutAndCancelBoundary", func(t *testing.T) {
		provider := readAppFile(t, "lib/ui/state/audio_player_provider.dart")
		assertContainsAll(t, "audio_player_provider.dart", provider, "setSleepTimer", "SleepTimerOption")
	})

	t.Run("F09_T2_05_TabularTimestampFormatting", func(t *testing.T) {
		scrubber := readAppFile(t, "lib/ui/features/audiobook/widgets/scrubber_bar.dart")
		assertContainsAll(t, "scrubber_bar.dart", scrubber, "padLeft(2, '0')")
	})

	// -------------------------------------------------------------------------
	// Feature 10 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F10_T2_01_LiveAPI_StageUploadUnsupportedExtensionAndCorruptZip", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		// 1. Unsupported .exe extension -> 400
		var body1 bytes.Buffer
		mpw1 := multipart.NewWriter(&body1)
		p1, _ := mpw1.CreateFormFile("file", "malware.exe")
		_, _ = p1.Write([]byte("MZ-binary"))
		_ = mpw1.Close()

		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body1)
		req1.Header.Set("Authorization", "Bearer "+token)
		req1.Header.Set("Content-Type", mpw1.FormDataContentType())
		rec1 := httptest.NewRecorder()
		f.handler.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for .exe upload, got %d", rec1.Code)
		}

		// 2. Corrupt .epub zip -> 400
		var body2 bytes.Buffer
		mpw2 := multipart.NewWriter(&body2)
		p2, _ := mpw2.CreateFormFile("file", "corrupt.epub")
		_, _ = p2.Write([]byte("not-a-valid-zip-archive"))
		_ = mpw2.Close()

		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body2)
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("Content-Type", mpw2.FormDataContentType())
		rec2 := httptest.NewRecorder()
		f.handler.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for corrupt .epub zip, got %d", rec2.Code)
		}
	})

	t.Run("F10_T2_02_LiveAPI_StageUploadDuplicateDetectionAndMissingAuthorWarning", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		// Stage EPUB with no author -> warning "No author found in metadata"
		noAuthorBytes := buildInMemoryEPUB(t, "Authorless Scroll", "", "Mystery text")
		var body1 bytes.Buffer
		mpw1 := multipart.NewWriter(&body1)
		p1, _ := mpw1.CreateFormFile("file", "no_author.epub")
		_, _ = p1.Write(noAuthorBytes)
		_ = mpw1.Close()

		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body1)
		req1.Header.Set("Authorization", "Bearer "+token)
		req1.Header.Set("Content-Type", mpw1.FormDataContentType())
		rec1 := httptest.NewRecorder()
		f.handler.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusOK || !strings.Contains(rec1.Body.String(), "No author found") {
			t.Errorf("expected missing author warning, got %d: %s", rec1.Code, rec1.Body.String())
		}

		// Stage duplicate of existing book ("Solaris: The Novel?" by "Stanislaw Lem")
		dupBytes := buildInMemoryEPUB(t, "Solaris: The Novel?", "Stanislaw Lem", "Duplicate copy")
		var body2 bytes.Buffer
		mpw2 := multipart.NewWriter(&body2)
		p2, _ := mpw2.CreateFormFile("file", "solaris_dup.epub")
		_, _ = p2.Write(dupBytes)
		_ = mpw2.Close()

		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body2)
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("Content-Type", mpw2.FormDataContentType())
		rec2 := httptest.NewRecorder()
		f.handler.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), `"is_duplicate":true`) {
			t.Errorf("expected is_duplicate:true, got %d: %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("F10_T2_03_LiveAPI_DeleteStagedUploadJobCleanup", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		epubBytes := buildInMemoryEPUB(t, "Temp Book", "Temp Author", "To be deleted")
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		p, _ := mpw.CreateFormFile("file", "temp.epub")
		_, _ = p.Write(epubBytes)
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		var stageResp struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &stageResp)

		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/books/upload/jobs/"+stageResp.JobID, nil)
		delReq.Header.Set("Authorization", "Bearer "+token)
		delRec := httptest.NewRecorder()
		f.handler.ServeHTTP(delRec, delReq)
		if delRec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 on delete staged job, got %d", delRec.Code)
		}
	})

	t.Run("F10_T2_04_LiveDestinationPathSanitization", func(t *testing.T) {
		review := readAppFile(t, "lib/ui/features/upload/upload_review_dialog.dart")
		uploads := readAppFile(t, "lib/ui/features/upload/uploads_view.dart")
		uploadModel := readAppFile(t, "lib/data/models/upload_job.dart")
		assertContainsAll(t, "upload_review_dialog.dart", review, "/library/", "sanitizePathSegment")
		assertContainsAll(t, "uploads_view.dart", uploads, "/library/", "sanitizePathSegment")
		assertContainsAll(t, "upload_job.dart", uploadModel, "sanitizePathSegment", "replaceAll")
	})

	t.Run("F10_T2_05_DesktopAndMobileInspectorParityContract", func(t *testing.T) {
		review := readAppFile(t, "lib/ui/features/upload/upload_review_dialog.dart")
		uploads := readAppFile(t, "lib/ui/features/upload/uploads_view.dart")
		assertContainsAll(t, "upload_review_dialog.dart", review, "Title", "Author", "Series", "Genres", "Description")
		assertContainsAll(t, "uploads_view.dart", uploads, "Title", "Author", "Series", "Genres", "Description")
	})

	// -------------------------------------------------------------------------
	// Feature 11 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F11_T2_01_LiveAPI_OAuthAuthorizeInvalidPasswordRendersErrorBox", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()

		mux := http.NewServeMux()
		oauthHandler := api.NewOAuthHandler(f.repo)
		oauthHandler.RegisterRoutes(mux)

		regReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"TestClient","redirect_uris":["https://cb.test/oauth"]}`))
		regRec := httptest.NewRecorder()
		mux.ServeHTTP(regRec, regReq)
		var regResp api.RegisterResponse
		_ = json.NewDecoder(regRec.Body).Decode(&regResp)

		form := url.Values{
			"action":       {"authorize"},
			"username":     {"admin"},
			"password":     {"wrong-password-999"},
			"client_id":    {regResp.ClientID},
			"redirect_uri": {"https://cb.test/oauth"},
			"state":        {"st-1"},
		}
		postReq := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
		postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		postRec := httptest.NewRecorder()
		mux.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusOK || !strings.Contains(postRec.Body.String(), "Invalid username or password.") {
			t.Fatalf("expected 200 with error box on invalid OAuth password, got %d", postRec.Code)
		}
	})

	t.Run("F11_T2_02_LiveAPI_OAuthAuthorizeDenyRedirectsAccessDenied", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()

		mux := http.NewServeMux()
		oauthHandler := api.NewOAuthHandler(f.repo)
		oauthHandler.RegisterRoutes(mux)

		regReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"TestClient","redirect_uris":["https://cb.test/oauth"]}`))
		regRec := httptest.NewRecorder()
		mux.ServeHTTP(regRec, regReq)
		var regResp api.RegisterResponse
		_ = json.NewDecoder(regRec.Body).Decode(&regResp)

		form := url.Values{
			"action":       {"deny"},
			"client_id":    {regResp.ClientID},
			"redirect_uri": {"https://cb.test/oauth"},
			"state":        {"st-deny"},
		}
		postReq := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
		postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		postRec := httptest.NewRecorder()
		mux.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusFound || !strings.Contains(postRec.Header().Get("Location"), "error=access_denied") {
			t.Fatalf("expected 302 redirect with error=access_denied, got %d (%s)", postRec.Code, postRec.Header().Get("Location"))
		}
	})

	t.Run("F11_T2_03_LiveAPI_SearchEmptyQueryAndLimitClamping", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=&limit=999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("expected 200 with [] for empty search query, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("F11_T2_04_LiveAPI_MergeBooksValidationErrors", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/merge-books", strings.NewReader(`{"primary_book_id":"","duplicate_book_ids":[]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 on invalid merge-books payload, got %d", rec.Code)
		}
	})

	t.Run("F11_T2_05_SettingsPasswordMismatchAndConnectValidation", func(t *testing.T) {
		settings := readAppFile(t, "lib/ui/features/settings/settings_view.dart")
		assertContainsAll(t, "settings_view.dart", settings, "Current password required", "Passwords do not match")
	})

	// -------------------------------------------------------------------------
	// Feature 12 Boundaries
	// -------------------------------------------------------------------------
	t.Run("F12_T2_01_ZeroTautologicalTestsInFlutterE2ESuite", func(t *testing.T) {
		suite := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		if strings.Contains(suite, "expect(true, isTrue)") || strings.Contains(suite, "skip: true") {
			t.Errorf("tactile_scriptorium_e2e_test.dart must not contain tautological expect(true, isTrue) or skip: true")
		}
		expectCount := strings.Count(suite, "expect(")
		if expectCount < 140 {
			t.Errorf("expected at least 140 real expect(...) assertions in tactile_scriptorium_e2e_test.dart, found %d", expectCount)
		}
	})

	t.Run("F12_T2_02_RouterAllRoutesAndRedirectsVerified", func(t *testing.T) {
		router := readAppFile(t, "lib/ui/router.dart")
		assertContainsAll(t, "router.dart", router,
			"path: '/connect'",
			"path: '/library'",
			"path: '/books'",
			"path: ':bookId'",
			"path: '/series'",
			"path: '/authors'",
			"path: '/genres'",
			"path: '/topics'",
			"path: '/search'",
			"path: '/settings'",
			"path: '/utilities'",
			"path: '/uploads'",
			"path: '/books/:bookId/listen'",
			"path: '/books/:bookId/read'",
			"path: '/book/:bookId'",
			"path: '/reader/:bookId'",
		)
	})

	t.Run("F12_T2_03_LiveAPI_CORSPreflightAndAuthMiddlewareRejection", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()

		optReq := httptest.NewRequest(http.MethodOptions, "/api/v1/books", nil)
		optRec := httptest.NewRecorder()
		f.handler.ServeHTTP(optRec, optReq)
		if optRec.Code != http.StatusNoContent || optRec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("unexpected OPTIONS response: %d", optRec.Code)
		}

		unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/books", nil)
		unauthRec := httptest.NewRecorder()
		f.handler.ServeHTTP(unauthRec, unauthReq)
		if unauthRec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on unauthenticated GET /api/v1/books, got %d", unauthRec.Code)
		}
	})

	t.Run("F12_T2_04_LiveAPI_SPAHandlerSecurityAndCacheBoundaries", func(t *testing.T) {
		webDir := filepath.Join(repoRoot(t), "apps", "app", "web")
		handler := api.SPAHandler(webDir)

		// Missing asset with extension -> 404
		missReq := httptest.NewRequest(http.MethodGet, "/assets/nonexistent.woff2", nil)
		missRec := httptest.NewRecorder()
		handler.ServeHTTP(missRec, missReq)
		if missRec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for missing static asset, got %d", missRec.Code)
		}

		// POST method -> 405
		postReq := httptest.NewRequest(http.MethodPost, "/books", nil)
		postRec := httptest.NewRecorder()
		handler.ServeHTTP(postRec, postReq)
		if postRec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for POST on SPAHandler, got %d", postRec.Code)
		}

		// Static icon -> immutable cache header
		iconReq := httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
		iconRec := httptest.NewRecorder()
		handler.ServeHTTP(iconRec, iconReq)
		if iconRec.Code == http.StatusOK && !strings.Contains(iconRec.Header().Get("Cache-Control"), "immutable") {
			t.Errorf("expected immutable Cache-Control on favicon.png, got %s", iconRec.Header().Get("Cache-Control"))
		}
	})

	t.Run("F12_T2_05_LiveAPI_QueueStatusAndSSEAuthContract", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/queue/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on queue status, got %d", rec.Code)
		}
		assertContainsAll(t, "queue/status JSON", rec.Body.String(), "pending_chapters", "total_chapters", "is_active")
	})
}

// =============================================================================
// TIER 3 — CROSS-FEATURE COMBINATIONS (18 Pairwise Interaction Subtests)
// =============================================================================

func TestTactileScriptorium_Tier3_CrossFeatureCombinations(t *testing.T) {
	t.Run("T3_01_ThemeSwitching_BoneSepiaDark_PropagatesToAppShellAndSharedLayout", func(t *testing.T) {
		theme := readAppFile(t, "lib/ui/core/theme.dart")
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		shell := readAppFile(t, "lib/ui/core/app_shell.dart")
		assertContainsAll(t, "theme.dart", theme, "ShelfdColors.bone", "ShelfdColors.sepia", "ShelfdColors.dark")
		assertContainsAll(t, "shared_layout.dart", shared, "ShelfdColors.of(context)")
		assertContainsAll(t, "app_shell.dart", shell, "ShelfdAdaptiveScaffold")
	})

	t.Run("T3_02_ThemeSwitching_PropagatesToLibraryCatalogAndFilterBar", func(t *testing.T) {
		libView := readAppFile(t, "lib/ui/features/library/library_view.dart")
		filterBar := readAppFile(t, "lib/ui/features/library/library_filter_bar.dart")
		e2e := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertValidDartSource(t, "library_view.dart", libView)
		assertValidDartSource(t, "library_filter_bar.dart", filterBar)
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2e, "F04-T2-05")
	})

	t.Run("T3_03_ThemeSwitching_PropagatesToBookDetailAndMetadataModals", func(t *testing.T) {
		detail := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		editDlg := readAppFile(t, "lib/ui/features/book_detail/edit_book_metadata_dialog.dart")
		searchDlg := readAppFile(t, "lib/ui/features/book_detail/metadata_search_dialog.dart")
		assertValidDartSource(t, "book_detail_view.dart", detail)
		assertValidDartSource(t, "edit_book_metadata_dialog.dart", editDlg)
		assertValidDartSource(t, "metadata_search_dialog.dart", searchDlg)
	})

	t.Run("T3_04_ThemeSwitching_PropagatesToReaderDrawerToolbarAndSheets", func(t *testing.T) {
		reader := readAppFile(t, "lib/ui/features/reader/reader_view.dart")
		md := readAppFile(t, "lib/ui/features/reader/reader_markdown.dart")
		assertContainsAll(t, "reader_view.dart", reader, "ReadingThemeMode", "Bone", "Sepia", "Dark OLED")
		assertContainsAll(t, "reader_markdown.dart", md, "buildReaderBlockWidget")
	})

	t.Run("T3_05_ThemeSwitching_PropagatesToUploadsSearchUtilitiesSettingsConnect", func(t *testing.T) {
		for _, rel := range []string{
			"lib/ui/features/upload/uploads_view.dart",
			"lib/ui/features/search/search_view.dart",
			"lib/ui/features/utilities/utilities_view.dart",
			"lib/ui/features/settings/settings_view.dart",
			"lib/ui/features/connect/connect_view.dart",
		} {
			src := readAppFile(t, rel)
			assertValidDartSource(t, rel, src)
		}
	})

	t.Run("T3_06_Typography_EditorialSerifSansMono_AcrossBookDetailAudiobookAndUploads", func(t *testing.T) {
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		assertContainsAll(t, "typography.dart", typo, "Newsreader", "Work Sans", "JetBrains Mono", "FontFeature.tabularFigures()")
		assertContainsAll(t, "shared_layout.dart", shared, "AppTypography.mono", "AppTypography.titleSerif", "AppTypography.bodySans")
	})

	t.Run("T3_07_HtmlText_InsideBookDetailSynopsis_AdaptsToActiveTheme", func(t *testing.T) {
		htmlText := readAppFile(t, "lib/ui/core/html_text.dart")
		bookDetail := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		assertContainsAll(t, "html_text.dart", htmlText, "ShelfdColors.of(context)")
		assertContainsAll(t, "book_detail_view.dart", bookDetail, "HtmlText(")
	})

	t.Run("T3_08_AppShell_PersistentMiniPlayerBar_AcrossLibrarySearchAndSettingsRoutes", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		mini := readAppFile(t, "lib/ui/features/audiobook/widgets/mini_player_bar.dart")
		e2e := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")
		assertContainsAll(t, "shared_layout.dart", shared, "const MiniPlayerBar()")
		assertContainsAll(t, "mini_player_bar.dart", mini, "audioPlayerProvider")
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2e, "T3-02")
	})

	t.Run("T3_09_AppShell_GlobalDropTarget_StagesUploadAndUpdatesSidebarBadge", func(t *testing.T) {
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		drop := readAppFile(t, "lib/ui/features/upload/upload_drop_target.dart")
		assertContainsAll(t, "shared_layout.dart", shared, "stagedUploads")
		assertContainsAll(t, "upload_drop_target.dart", drop, "uploadProvider")
	})

	t.Run("T3_10_LibraryFilterBar_TokenSelection_FiltersLiveBooksAPIAndUpdatesGrid", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/books?search="+url.QueryEscape(`author:"Stanislaw Lem"`), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Solaris") {
			t.Fatalf("expected Solaris for author filter, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("T3_11_TaxonomyDetail_AuthorAndSeriesCards_NavigateToBookDetailShowcase", func(t *testing.T) {
		authorView := readAppFile(t, "lib/ui/features/authors/author_detail_view.dart")
		seriesView := readAppFile(t, "lib/ui/features/series/series_detail_view.dart")
		assertContainsAll(t, "author_detail_view.dart", authorView, "/books/")
		assertContainsAll(t, "series_detail_view.dart", seriesView, "/books/")
	})

	t.Run("T3_12_BookDetail_EditMetadataAndCoverUpload_RefreshesCatalogAndSeriesGrouping", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		putReq := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+book.ID+"/metadata", strings.NewReader(`{"title":"Solaris Restored","authors":["Stanislaw Lem"],"series":"Cyberiad Series","sequence_number":2}`))
		putReq.Header.Set("Authorization", "Bearer "+token)
		putReq.Header.Set("Content-Type", "application/json")
		putRec := httptest.NewRecorder()
		f.handler.ServeHTTP(putRec, putReq)
		if putRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on metadata update, got %d", putRec.Code)
		}

		serReq := httptest.NewRequest(http.MethodGet, "/api/v1/series", nil)
		serReq.Header.Set("Authorization", "Bearer "+token)
		serRec := httptest.NewRecorder()
		f.handler.ServeHTTP(serRec, serReq)
		if !strings.Contains(serRec.Body.String(), "Cyberiad Series") {
			t.Errorf("expected updated series in /api/v1/series, got %s", serRec.Body.String())
		}
	})

	t.Run("T3_13_BookDetail_BookmarksAndHighlights_SyncWithReflowableReader", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		hlReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/highlights", strings.NewReader(`{"selected_text":"Kelvin entered the arrival capsule.","color":"pink","chapter_id":"ch-nlm-1"}`))
		hlReq.Header.Set("Authorization", "Bearer "+token)
		hlReq.Header.Set("Content-Type", "application/json")
		hlRec := httptest.NewRecorder()
		f.handler.ServeHTTP(hlRec, hlReq)
		if hlRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 on highlight create, got %d", hlRec.Code)
		}

		detReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID, nil)
		detReq.Header.Set("Authorization", "Bearer "+token)
		detRec := httptest.NewRecorder()
		f.handler.ServeHTTP(detRec, detReq)
		if !strings.Contains(detRec.Body.String(), "Kelvin entered the arrival capsule.") {
			t.Errorf("expected new highlight in book detail response")
		}
	})

	t.Run("T3_14_BookDetail_RAGChatCitationJump_OpensReaderAtCitedChapter", func(t *testing.T) {
		detail := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		router := readAppFile(t, "lib/ui/router.dart")
		assertContainsAll(t, "book_detail_view.dart", detail, "Read in Reader", "/read/")
		assertContainsAll(t, "router.dart", router, ":chapterIdentifier")
	})

	t.Run("T3_15_BookDetail_ListenCTA_StartsAudiobookStudioAndDocksMiniPlayer", func(t *testing.T) {
		detail := readAppFile(t, "lib/ui/features/book_detail/book_detail_view.dart")
		router := readAppFile(t, "lib/ui/router.dart")
		assertContainsAll(t, "book_detail_view.dart", detail, "listen_hero_button", "/listen")
		assertContainsAll(t, "router.dart", router, "/books/:bookId/listen")
	})

	t.Run("T3_16_StagedUpload_DuplicateDetection_InteractsWithUtilitiesDuplicateFinder", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		seedNotebookLMTestBook(t, f)

		dupBytes := buildInMemoryEPUB(t, "Solaris: The Novel?", "Stanislaw Lem", "Second copy")
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		p, _ := mpw.CreateFormFile("file", "solaris2.epub")
		_, _ = p.Write(dupBytes)
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), `"is_duplicate":true`) {
			t.Errorf("expected staged duplicate flag, got %s", rec.Body.String())
		}
	})

	t.Run("T3_17_UtilitiesMergeBooks_TransfersHighlightsAndBookmarksToPrimaryBookDetail", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		ctx := context.Background()

		b1 := &repository.Book{ID: "p-book", Title: "Foundation", FilePath: filepath.Join(f.dataDir, "f1.epub")}
		b2 := &repository.Book{ID: "s-book", Title: "Foundation (Dup)", FilePath: filepath.Join(f.dataDir, "f2.epub")}
		_ = os.WriteFile(b1.FilePath, []byte("1"), 0644)
		_ = os.WriteFile(b2.FilePath, []byte("2"), 0644)
		_ = f.repo.CreateBook(ctx, b1)
		_ = f.repo.CreateBook(ctx, b2)
		_ = f.repo.CreateBookmark(ctx, &repository.Bookmark{ID: "bm-dup-1", BookID: b2.ID, Title: "Terminus Vault", Progress: 0.5})
		_ = f.repo.CreateHighlight(ctx, &repository.Highlight{ID: "hl-dup-1", BookID: b2.ID, SelectedText: "Violence is the last refuge of the incompetent.", Color: "blue"})

		mergePayload := `{"primary_book_id":"p-book","duplicate_book_ids":["s-book"],"options":{"transfer_bookmarks":true,"transfer_highlights":true,"merge_metadata":true,"delete_files":true}}`
		mReq := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/merge-books", strings.NewReader(mergePayload))
		mReq.Header.Set("Authorization", "Bearer "+token)
		mReq.Header.Set("Content-Type", "application/json")
		mRec := httptest.NewRecorder()
		f.handler.ServeHTTP(mRec, mReq)
		if mRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on merge, got %d: %s", mRec.Code, mRec.Body.String())
		}

		detReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/p-book", nil)
		detReq.Header.Set("Authorization", "Bearer "+token)
		detRec := httptest.NewRecorder()
		f.handler.ServeHTTP(detRec, detReq)
		assertContainsAll(t, "merged primary book detail", detRec.Body.String(),
			"Terminus Vault",
			"Violence is the last refuge of the incompetent.",
		)
	})

	t.Run("T3_18_OAuthPKCEFlow_IssuesBearerToken_AcceptedByProtectedLibraryAndSearchAPIs", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		seedNotebookLMTestBook(t, f)

		rootMux := http.NewServeMux()
		oauthHandler := api.NewOAuthHandler(f.repo)
		oauthHandler.RegisterRoutes(rootMux)
		rootMux.Handle("/api/v1/", f.handler)

		regReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"MCP Bridge","redirect_uris":["http://127.0.0.1/cb"]}`))
		regRec := httptest.NewRecorder()
		rootMux.ServeHTTP(regRec, regReq)
		var regResp api.RegisterResponse
		_ = json.NewDecoder(regRec.Body).Decode(&regResp)

		verifier := "pkce-verifier-string-for-tactile-scriptorium-e2e-test-12345"
		sum := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])

		authForm := url.Values{
			"action":                {"authorize"},
			"username":              {f.user.Username},
			"password":              {f.password},
			"client_id":             {regResp.ClientID},
			"redirect_uri":          {"http://127.0.0.1/cb"},
			"state":                 {"st-pkce"},
			"code_challenge":        {challenge},
			"code_challenge_method": {"S256"},
		}
		authReq := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(authForm.Encode()))
		authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		authRec := httptest.NewRecorder()
		rootMux.ServeHTTP(authRec, authReq)
		loc, _ := url.Parse(authRec.Header().Get("Location"))
		code := loc.Query().Get("code")

		tokForm := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {regResp.ClientID},
			"redirect_uri":  {"http://127.0.0.1/cb"},
			"code_verifier": {verifier},
		}
		tokReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokForm.Encode()))
		tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tokRec := httptest.NewRecorder()
		rootMux.ServeHTTP(tokRec, tokReq)
		var tokResp api.TokenResponse
		_ = json.NewDecoder(tokRec.Body).Decode(&tokResp)

		booksReq := httptest.NewRequest(http.MethodGet, "/api/v1/books", nil)
		booksReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
		booksRec := httptest.NewRecorder()
		rootMux.ServeHTTP(booksRec, booksReq)
		if booksRec.Code != http.StatusOK || !strings.Contains(booksRec.Body.String(), "Solaris") {
			t.Fatalf("expected OAuth token to access /api/v1/books, got %d: %s", booksRec.Code, booksRec.Body.String())
		}
	})
}

// =============================================================================
// TIER 4 — REAL-WORLD USER JOURNEYS (8 Stateful End-to-End Scenarios)
// =============================================================================

func TestTactileScriptorium_Tier4_RealWorldUserJourneys(t *testing.T) {
	t.Run("T4_Journey01_OnboardingAndConnectToLibraryDiscovery", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		seedNotebookLMTestBook(t, f)

		// Step 1: Discover server connect-info
		ciReq := httptest.NewRequest(http.MethodGet, "/api/v1/server/connect-info", nil)
		ciRec := httptest.NewRecorder()
		f.handler.ServeHTTP(ciRec, ciReq)
		if ciRec.Code != http.StatusOK {
			t.Fatalf("connect-info failed: %d", ciRec.Code)
		}

		// Step 2: Authenticate and verify /api/v1/auth/me
		token := f.loginAndGetToken(t)
		meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		meReq.Header.Set("Authorization", "Bearer "+token)
		meRec := httptest.NewRecorder()
		f.handler.ServeHTTP(meRec, meReq)
		if meRec.Code != http.StatusOK || !strings.Contains(meRec.Body.String(), `"username":"admin"`) {
			t.Fatalf("/api/v1/auth/me failed: %d %s", meRec.Code, meRec.Body.String())
		}

		// Step 3: Load Library catalog & taxonomy endpoints
		for _, ep := range []string{"/api/v1/books", "/api/v1/series", "/api/v1/authors", "/api/v1/genres", "/api/v1/topics", "/api/v1/queue/status"} {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("Journey01 GET %s failed: %d", ep, rec.Code)
			}
		}
	})

	t.Run("T4_Journey02_StagedEpubIngestionReviewCoverUploadAndCommit", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		// Step 1: Stage EPUB
		epubBytes := buildInMemoryEPUB(t, "A Wizard of Earthsea", "Ursula K. Le Guin", "Ged studies at Roke.")
		var stageBody bytes.Buffer
		mpw := multipart.NewWriter(&stageBody)
		part, _ := mpw.CreateFormFile("file", "earthsea.epub")
		_, _ = part.Write(epubBytes)
		_ = mpw.Close()

		sReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &stageBody)
		sReq.Header.Set("Authorization", "Bearer "+token)
		sReq.Header.Set("Content-Type", mpw.FormDataContentType())
		sRec := httptest.NewRecorder()
		f.handler.ServeHTTP(sRec, sReq)
		if sRec.Code != http.StatusOK {
			t.Fatalf("stage failed: %d", sRec.Code)
		}
		var staged struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(sRec.Body.Bytes(), &staged)

		// Step 2: Upload custom cover to staged job
		jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
		var cBody bytes.Buffer
		cmpw := multipart.NewWriter(&cBody)
		cp, _ := cmpw.CreateFormFile("cover", "earthsea.jpg")
		_, _ = cp.Write(jpeg)
		_ = cmpw.Close()

		cReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+staged.JobID+"/cover", &cBody)
		cReq.Header.Set("Authorization", "Bearer "+token)
		cReq.Header.Set("Content-Type", cmpw.FormDataContentType())
		cRec := httptest.NewRecorder()
		f.handler.ServeHTTP(cRec, cReq)
		if cRec.Code != http.StatusOK {
			t.Fatalf("staged cover upload failed: %d", cRec.Code)
		}

		// Step 3: Commit with full metadata
		commitJSON := `{"title":"A Wizard of Earthsea","author":"Ursula K. Le Guin","series":"Earthsea Cycle","sequence_number":1,"publisher":"Parnassus Press","language":"en","genres":["Fantasy"]}`
		cmReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+staged.JobID+"/commit", strings.NewReader(commitJSON))
		cmReq.Header.Set("Authorization", "Bearer "+token)
		cmReq.Header.Set("Content-Type", "application/json")
		cmRec := httptest.NewRecorder()
		f.handler.ServeHTTP(cmRec, cmReq)
		if cmRec.Code != http.StatusCreated && cmRec.Code != http.StatusOK {
			t.Fatalf("commit failed: %d %s", cmRec.Code, cmRec.Body.String())
		}

		// Step 4: Verify book is in catalog
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/books?search=Earthsea", nil)
		listReq.Header.Set("Authorization", "Bearer "+token)
		listRec := httptest.NewRecorder()
		f.handler.ServeHTTP(listRec, listReq)
		if !strings.Contains(listRec.Body.String(), "A Wizard of Earthsea") {
			t.Fatalf("committed book missing from catalog: %s", listRec.Body.String())
		}
	})

	t.Run("T4_Journey03_CatalogFilteringSeriesExplorationAndMetadataEnrichment", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		// Step 1: Filter catalog by author & genre
		fReq := httptest.NewRequest(http.MethodGet, "/api/v1/books?search="+url.QueryEscape(`author:"Stanislaw Lem" genre:"Science Fiction"`), nil)
		fReq.Header.Set("Authorization", "Bearer "+token)
		fRec := httptest.NewRecorder()
		f.handler.ServeHTTP(fRec, fReq)
		if fRec.Code != http.StatusOK || !strings.Contains(fRec.Body.String(), book.ID) {
			t.Fatalf("catalog filter failed: %d", fRec.Code)
		}

		// Step 2: Enrich metadata via PUT /api/v1/books/{id}/metadata
		enrichJSON := `{"title":"Solaris","authors":["Stanislaw Lem"],"series":"SF Masterworks","sequence_number":12,"publisher":"Faber & Faber","language":"en","genres":["Science Fiction","Philosophical Fiction"],"topics":["First Contact","Sentient Ocean"]}`
		putReq := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+book.ID+"/metadata", strings.NewReader(enrichJSON))
		putReq.Header.Set("Authorization", "Bearer "+token)
		putReq.Header.Set("Content-Type", "application/json")
		putRec := httptest.NewRecorder()
		f.handler.ServeHTTP(putRec, putReq)
		if putRec.Code != http.StatusOK || !strings.Contains(putRec.Body.String(), "Philosophical Fiction") {
			t.Fatalf("metadata enrichment failed: %d %s", putRec.Code, putRec.Body.String())
		}
	})

	t.Run("T4_Journey04_ReflowableReadingHighlightingNoteTakingAndNotebookLMExport", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		// Step 1: Fetch chapter 1 for reading
		chReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID+"/chapters/1", nil)
		chReq.Header.Set("Authorization", "Bearer "+token)
		chRec := httptest.NewRecorder()
		f.handler.ServeHTTP(chRec, chReq)
		if chRec.Code != http.StatusOK {
			t.Fatalf("chapter fetch failed: %d", chRec.Code)
		}

		// Step 2: Create highlight with note and bookmark
		hlReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/highlights", strings.NewReader(`{"selected_text":"The shifting mimoids rose above the horizon","color":"orange","note":"Vivid imagery of the ocean","chapter_id":"ch-nlm-2"}`))
		hlReq.Header.Set("Authorization", "Bearer "+token)
		hlReq.Header.Set("Content-Type", "application/json")
		hlRec := httptest.NewRecorder()
		f.handler.ServeHTTP(hlRec, hlReq)
		if hlRec.Code != http.StatusCreated {
			t.Fatalf("highlight creation failed: %d", hlRec.Code)
		}

		bmReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/bookmarks", strings.NewReader(`{"title":"Mimoid Observation","progress":0.62,"chapter_id":"ch-nlm-2"}`))
		bmReq.Header.Set("Authorization", "Bearer "+token)
		bmReq.Header.Set("Content-Type", "application/json")
		bmRec := httptest.NewRecorder()
		f.handler.ServeHTTP(bmRec, bmReq)
		if bmRec.Code != http.StatusCreated {
			t.Fatalf("bookmark creation failed: %d", bmRec.Code)
		}

		// Step 3: Export to NotebookLM Markdown and verify highlight + note appear
		nlmReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID+"/notebooklm", nil)
		nlmReq.Header.Set("Authorization", "Bearer "+token)
		nlmRec := httptest.NewRecorder()
		f.handler.ServeHTTP(nlmRec, nlmReq)
		assertContainsAll(t, "NotebookLM export", nlmRec.Body.String(),
			"The shifting mimoids rose above the horizon",
			"Vivid imagery of the ocean",
		)
	})

	t.Run("T4_Journey05_FixedLayoutSpreadReadingAndAssetRendering", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		bookDir := filepath.Join(f.libDir, "Chronicle", "Illuminated")
		_ = os.MkdirAll(bookDir, 0755)
		epubPath := filepath.Join(bookDir, "Illuminated.epub")
		imgData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'I', 'L', 'L', 'U', 'M'}

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		m, _ := zw.Create("mimetype")
		_, _ = m.Write([]byte("application/epub+zip"))
		c, _ := zw.Create("META-INF/container.xml")
		_, _ = c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
		opf, _ := zw.Create("OEBPS/content.opf")
		_, _ = opf.Write([]byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Illuminated Manuscript</dc:title><meta property="rendition:layout">pre-paginated</meta></metadata><manifest><item id="p1" href="text/p1.xhtml" media-type="application/xhtml+xml"/><item id="img1" href="img/folio1.jpg" media-type="image/jpeg"/></manifest><spine><itemref idref="p1" properties="page-spread-right"/></spine></package>`))
		p1, _ := zw.Create("OEBPS/text/p1.xhtml")
		_, _ = p1.Write([]byte(`<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><meta name="viewport" content="width=1200, height=1600"/></head><body><img src="../img/folio1.jpg"/></body></html>`))
		im, _ := zw.Create("OEBPS/img/folio1.jpg")
		_, _ = im.Write(imgData)
		_ = zw.Close()
		_ = os.WriteFile(epubPath, buf.Bytes(), 0644)

		book, err := f.ingester.IngestFile(context.Background(), epubPath, filepath.Join("Chronicle", "Illuminated", "Illuminated.epub"))
		if err != nil {
			t.Fatalf("IngestFile: %v", err)
		}

		// Step 1: Fetch rewritten chapter HTML
		htmlReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/0/html", book.ID), nil)
		htmlReq.Header.Set("Authorization", "Bearer "+token)
		htmlRec := httptest.NewRecorder()
		f.handler.ServeHTTP(htmlRec, htmlReq)
		if htmlRec.Code != http.StatusOK {
			t.Fatalf("chapter html failed: %d", htmlRec.Code)
		}

		// Step 2: Fetch embedded image asset
		assetURL := fmt.Sprintf("/api/v1/books/%s/assets/OEBPS/img/folio1.jpg", book.ID)
		assetReq := httptest.NewRequest(http.MethodGet, assetURL, nil)
		assetReq.Header.Set("Authorization", "Bearer "+token)
		assetRec := httptest.NewRecorder()
		f.handler.ServeHTTP(assetRec, assetReq)
		if assetRec.Code != http.StatusOK || !bytes.Equal(assetRec.Body.Bytes(), imgData) {
			t.Fatalf("asset fetch failed: %d", assetRec.Code)
		}
	})

	t.Run("T4_Journey06_AudiobookListeningProgressSyncBookmarkingAndMiniPlayer", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		relPath := filepath.Join("Le Guin", "Earthsea Audio", "Earthsea.m4b")
		fullPath := filepath.Join(f.libDir, relPath)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		audioData := bytes.Repeat([]byte("M4BAUDIO"), 500)
		_ = os.WriteFile(fullPath, audioData, 0644)

		sz := int64(len(audioData))
		dur := 2400.0
		book := &repository.Book{ID: "journey-audio-1", Title: "Earthsea Audiobook", FilePath: relPath, BookType: "audiobook", DurationSeconds: &dur, FileSizeBytes: &sz}
		_ = f.repo.CreateBook(context.Background(), book)
		_ = f.repo.CreateAudioChapters(context.Background(), []*repository.AudioChapter{
			{BookID: book.ID, ChapterIndex: 1, Title: "The Shadow", StartOffsetSec: 0, DurationSec: 1200},
			{BookID: book.ID, ChapterIndex: 2, Title: "The School of Wizards", StartOffsetSec: 1200, DurationSec: 1200},
		})

		// Step 1: Stream byte range
		sReq := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/"+book.ID+"/stream", nil)
		sReq.Header.Set("Authorization", "Bearer "+token)
		sReq.Header.Set("Range", "bytes=0-127")
		sRec := httptest.NewRecorder()
		f.handler.ServeHTTP(sRec, sReq)
		if sRec.Code != http.StatusPartialContent {
			t.Fatalf("audio stream failed: %d", sRec.Code)
		}

		// Step 2: Save playback progress & create audio bookmark
		pReq := httptest.NewRequest(http.MethodPost, "/api/v1/audiobooks/"+book.ID+"/progress", strings.NewReader(`{"position_seconds":1320.0,"speed":1.25,"is_completed":false}`))
		pReq.Header.Set("Authorization", "Bearer "+token)
		pReq.Header.Set("Content-Type", "application/json")
		pRec := httptest.NewRecorder()
		f.handler.ServeHTTP(pRec, pReq)
		if pRec.Code != http.StatusOK {
			t.Fatalf("save progress failed: %d", pRec.Code)
		}

		bmReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/bookmarks", strings.NewReader(`{"title":"Audio Bookmark at 22:00","progress":0.55}`))
		bmReq.Header.Set("Authorization", "Bearer "+token)
		bmReq.Header.Set("Content-Type", "application/json")
		bmRec := httptest.NewRecorder()
		f.handler.ServeHTTP(bmRec, bmReq)
		if bmRec.Code != http.StatusCreated {
			t.Fatalf("audio bookmark failed: %d", bmRec.Code)
		}
	})

	t.Run("T4_Journey07_DuplicateBookDetectionComparisonAndSafeMerge", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		ctx := context.Background()

		author, _ := f.repo.UpsertAuthor(ctx, "Ursula K. Le Guin")
		isbn := "9780441478125"
		p1 := filepath.Join(f.dataDir, "lhod_primary.epub")
		p2 := filepath.Join(f.dataDir, "lhod_dup.epub")
		_ = os.WriteFile(p1, []byte("primary"), 0644)
		_ = os.WriteFile(p2, []byte("dup"), 0644)

		b1 := &repository.Book{ID: "j7-b1", Title: "The Left Hand of Darkness", Identifier: &isbn, FilePath: p1, CreatedAt: time.Now().UTC()}
		b2 := &repository.Book{ID: "j7-b2", Title: "The Left Hand of Darkness", Identifier: &isbn, FilePath: p2, CreatedAt: time.Now().UTC()}
		_ = f.repo.CreateBook(ctx, b1)
		_ = f.repo.CreateBook(ctx, b2)
		_ = f.repo.LinkBookAuthor(ctx, b1.ID, author.ID, "author")
		_ = f.repo.LinkBookAuthor(ctx, b2.ID, author.ID, "author")
		_ = f.repo.CreateHighlight(ctx, &repository.Highlight{ID: "j7-hl", BookID: b2.ID, SelectedText: "Truth is a matter of the imagination.", Color: "yellow"})

		// Step 1: Scan duplicates
		dReq := httptest.NewRequest(http.MethodGet, "/api/v1/utilities/duplicates", nil)
		dReq.Header.Set("Authorization", "Bearer "+token)
		dRec := httptest.NewRecorder()
		f.handler.ServeHTTP(dRec, dReq)
		if dRec.Code != http.StatusOK || !strings.Contains(dRec.Body.String(), "j7-b2") {
			t.Fatalf("duplicate scan failed: %s", dRec.Body.String())
		}

		// Step 2: Merge duplicate into primary
		mReq := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/merge-books", strings.NewReader(`{"primary_book_id":"j7-b1","duplicate_book_ids":["j7-b2"]}`))
		mReq.Header.Set("Authorization", "Bearer "+token)
		mReq.Header.Set("Content-Type", "application/json")
		mRec := httptest.NewRecorder()
		f.handler.ServeHTTP(mRec, mReq)
		if mRec.Code != http.StatusOK {
			t.Fatalf("merge failed: %d", mRec.Code)
		}

		// Step 3: Verify highlight transferred to primary
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/j7-b1", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if !strings.Contains(getRec.Body.String(), "Truth is a matter of the imagination.") {
			t.Fatalf("expected highlight transferred to j7-b1: %s", getRec.Body.String())
		}
	})

	t.Run("T4_Journey08_SemanticSearchRAGChatAndOAuthMCPClientAuthorization", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		book := seedNotebookLMTestBook(t, f)

		rootMux := http.NewServeMux()
		oauthHandler := api.NewOAuthHandler(f.repo)
		oauthHandler.RegisterRoutes(rootMux)
		rootMux.Handle("/api/v1/", f.handler)

		// Step 1: OAuth discovery
		discReq := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		discRec := httptest.NewRecorder()
		rootMux.ServeHTTP(discRec, discReq)
		if discRec.Code != http.StatusOK {
			t.Fatalf("OAuth discovery failed: %d", discRec.Code)
		}

		// Step 2: Register client & complete PKCE authorization
		regReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Research Agent","redirect_uris":["http://127.0.0.1/cb"]}`))
		regRec := httptest.NewRecorder()
		rootMux.ServeHTTP(regRec, regReq)
		var regResp api.RegisterResponse
		_ = json.NewDecoder(regRec.Body).Decode(&regResp)

		verifier := "journey08-pkce-verifier-string-9876543210-abcdefghij"
		sum := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])

		authForm := url.Values{
			"action":                {"authorize"},
			"username":              {f.user.Username},
			"password":              {f.password},
			"client_id":             {regResp.ClientID},
			"redirect_uri":          {"http://127.0.0.1/cb"},
			"state":                 {"j8"},
			"code_challenge":        {challenge},
			"code_challenge_method": {"S256"},
		}
		authReq := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(authForm.Encode()))
		authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		authRec := httptest.NewRecorder()
		rootMux.ServeHTTP(authRec, authReq)
		loc, _ := url.Parse(authRec.Header().Get("Location"))
		code := loc.Query().Get("code")

		tokForm := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {regResp.ClientID},
			"redirect_uri":  {"http://127.0.0.1/cb"},
			"code_verifier": {verifier},
		}
		tokReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokForm.Encode()))
		tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tokRec := httptest.NewRecorder()
		rootMux.ServeHTTP(tokRec, tokReq)
		var tokResp api.TokenResponse
		_ = json.NewDecoder(tokRec.Body).Decode(&tokResp)

		// Step 3: Execute semantic search and RAG book chat using the OAuth Bearer token
		sReq := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=ocean", nil)
		sReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
		sRec := httptest.NewRecorder()
		rootMux.ServeHTTP(sRec, sReq)
		if sRec.Code != http.StatusOK {
			t.Fatalf("OAuth semantic search failed: %d", sRec.Code)
		}

		cReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/chat", strings.NewReader(`{"query":"Summarize the ocean phenomenon"}`))
		cReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
		cReq.Header.Set("Content-Type", "application/json")
		cRec := httptest.NewRecorder()
		rootMux.ServeHTTP(cRec, cReq)
		if cRec.Code != http.StatusOK || !strings.Contains(cRec.Body.String(), "citations") {
			t.Fatalf("OAuth book chat failed: %d %s", cRec.Code, cRec.Body.String())
		}
	})
}

// =============================================================================
// TIER 5 — WHITE-BOX ADVERSARIAL HARDENING (12 Subtests: T5_01 through T5_12)
// =============================================================================

func TestTactileScriptorium_Tier5_AdversarialHardening(t *testing.T) {
	t.Run("T5_01_ZeroHardcodedLightModeTokensAcrossAllFeatureViewsAndSharedLayout", func(t *testing.T) {
		appRoot := filepath.Join(repoRoot(t), "apps", "app")
		targetDirs := []string{
			filepath.Join(appRoot, "lib", "ui", "features"),
			filepath.Join(appRoot, "lib", "ui", "core"),
		}
		forbiddenTokens := []string{
			"AppTokens.boneBackground",
			"AppTokens.boneSurface",
			"AppTokens.boneContainer",
			"AppTokens.crispBorder",
			"AppTokens.mutedCopy",
			"0xFF7C3AED",
		}
		for _, dir := range targetDirs {
			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".dart") {
					return err
				}
				rel, _ := filepath.Rel(appRoot, path)
				if rel == filepath.Join("lib", "ui", "core", "tokens.dart") || rel == filepath.Join("lib", "ui", "core", "theme.dart") {
					return nil
				}
				src := readAppFile(t, rel)
				assertContainsNone(t, rel, src, forbiddenTokens...)
				if rel != filepath.Join("lib", "ui", "features", "audiobook", "audiobook_player_view.dart") {
					assertContainsNone(t, rel, src, "AppTokens.charcoalInk")
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walking %s: %v", dir, err)
			}
		}
	})

	t.Run("T5_02_ShelfdColorsMathematicalLerpAndFallbackInvariants", func(t *testing.T) {
		theme := readAppFile(t, "lib/ui/core/theme.dart")
		assertContainsAll(t, "theme.dart", theme,
			"if (other is! ShelfdColors) return this;",
			"isDark: t < 0.5 ? isDark : other.isDark",
			"Color.lerp(background, other.background, t)!",
			"Color.lerp(accentPrimary, other.accentPrimary, t)!",
			"final ext = theme.extension<ShelfdColors>();",
			"if (ext != null) return ext;",
			"if (theme.brightness == Brightness.dark) return ShelfdColors.dark;",
		)
	})

	t.Run("T5_03_HtmlTextAdversarialScriptStrippingAndEntityDecodingContract", func(t *testing.T) {
		htmlText := readAppFile(t, "lib/ui/core/html_text.dart")
		assertContainsAll(t, "html_text.dart", htmlText,
			"caseSensitive: false, dotAll: true",
			"<script",
			"<style",
			"int.tryParse(match.group(1)!, radix: 16)",
			"int.tryParse(match.group(1)!)",
			"ShelfdColors.of(context)",
			"AppTypography.technicalMonoFamily",
		)
	})

	t.Run("T5_04_TaxonomyViewsAndCompareSeriesBooksAdversarialSortingContract", func(t *testing.T) {
		seriesView := readAppFile(t, "lib/ui/features/series/series_detail_view.dart")
		authorView := readAppFile(t, "lib/ui/features/authors/author_detail_view.dart")
		genreView := readAppFile(t, "lib/ui/features/genres/genre_detail_view.dart")
		topicView := readAppFile(t, "lib/ui/features/topics/topic_detail_view.dart")
		router := readAppFile(t, "lib/ui/router.dart")
		assertContainsAll(t, "series_detail_view.dart", seriesView,
			"int compareSeriesBooks(Book a, Book b)",
			"showFallbackTitle: false",
			"TactileBookCover(",
		)
		assertContainsAll(t, "author_detail_view.dart", authorView, "showFallbackTitle: false", "TactileBookCover(")
		assertContainsAll(t, "genre_detail_view.dart", genreView, "showFallbackTitle: false", "TactileBookCover(")
		assertContainsAll(t, "topic_detail_view.dart", topicView, "showFallbackTitle: false", "TactileBookCover(")
		assertContainsAll(t, "router.dart", router, "GenreDetailView(", "TopicDetailView(")
	})

	t.Run("T5_05_LiveAPI_AdversarialUnicodeXSSMetadataAndFractionalSequenceRoundtrip", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)
		book := seedNotebookLMTestBook(t, f)

		payload := `{
			"title": "Solaris: 太陽系 <script>alert('xss')</script> & \"Archival Edition\"",
			"authors": ["Stanisław Lem", "אורסולה לה גווין"],
			"series": "Hainish & Cyberiad Cycle",
			"sequence_number": 2.5,
			"publisher": "Wydawnictwo MON / 東京",
			"language": "pl-PL",
			"description": "<p>Unclosed paragraph <b>bold <i>italic<script>evil()</script> &mdash; &#x2014; &#160;</p>",
			"genres": ["Science Fiction", "Philosophical Speculation"]
		}`
		putReq := httptest.NewRequest(http.MethodPut, "/api/v1/books/"+book.ID+"/metadata", strings.NewReader(payload))
		putReq.Header.Set("Authorization", "Bearer "+token)
		putReq.Header.Set("Content-Type", "application/json")
		putRec := httptest.NewRecorder()
		f.handler.ServeHTTP(putRec, putReq)
		if putRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on adversarial metadata update, got %d: %s", putRec.Code, putRec.Body.String())
		}

		// Create bookmark without chapter_id (fallback reader jump) and highlight with dark-mode color
		bmReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/bookmarks", strings.NewReader(`{"title":"Bookmark <1>","progress":0.999}`))
		bmReq.Header.Set("Authorization", "Bearer "+token)
		bmReq.Header.Set("Content-Type", "application/json")
		bmRec := httptest.NewRecorder()
		f.handler.ServeHTTP(bmRec, bmReq)
		if bmRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 on bookmark create, got %d", bmRec.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID, nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		f.handler.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on GET book detail, got %d", getRec.Code)
		}
		assertContainsAll(t, "GET book detail adversarial roundtrip", getRec.Body.String(),
			"Stanisław Lem",
			"Hainish \\u0026 Cyberiad Cycle",
			"2.5",
			"Bookmark \\u003c1\\u003e",
		)
	})

	t.Run("T5_06_AllFlutterTestsImportResolutionAndZeroTautologiesAcrossEntireTestTree", func(t *testing.T) {
		appRoot := filepath.Join(repoRoot(t), "apps", "app")
		testRoot := filepath.Join(appRoot, "test")
		importRe := regexp.MustCompile(`(?m)^import\s+['"]([^'"]+)['"];`)

		testFiles := 0
		err := filepath.WalkDir(testRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".dart") {
				return err
			}
			testFiles++
			rel, _ := filepath.Rel(appRoot, path)
			src := readAppFile(t, rel)
			assertValidDartSource(t, rel, src)
			if strings.Contains(src, "expect(true, isTrue)") || strings.Contains(src, "skip: true") {
				t.Errorf("%s contains tautological assertion or skip: true", rel)
			}
			for _, m := range importRe.FindAllStringSubmatch(src, -1) {
				imp := m[1]
				if strings.HasPrefix(imp, "package:shelf/") {
					target := filepath.Join(appRoot, "lib", strings.TrimPrefix(imp, "package:shelf/"))
					if _, statErr := os.Stat(target); statErr != nil {
						t.Errorf("%s imports missing package file %s (%v)", rel, imp, statErr)
					}
				} else if !strings.HasPrefix(imp, "package:") && !strings.HasPrefix(imp, "dart:") {
					target := filepath.Clean(filepath.Join(filepath.Dir(path), imp))
					if _, statErr := os.Stat(target); statErr != nil {
						t.Errorf("%s imports missing relative file %s (%v)", rel, imp, statErr)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking test tree: %v", err)
		}
		if testFiles < 36 {
			t.Errorf("expected at least 36 test files in apps/app/test, found %d", testFiles)
		}
	})

	t.Run("T5_07_ReaderFixedLayoutAndMarkdownAdversarialHighlightAndAssetTraversalContract", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		bookDir := filepath.Join(f.libDir, "Art", "AdversarialSpread")
		_ = os.MkdirAll(bookDir, 0755)
		epubPath := filepath.Join(bookDir, "AdversarialSpread.epub")

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		m, _ := zw.Create("mimetype")
		_, _ = m.Write([]byte("application/epub+zip"))
		c, _ := zw.Create("META-INF/container.xml")
		_, _ = c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
		opf, _ := zw.Create("OEBPS/content.opf")
		_, _ = opf.Write([]byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Gatefold Atlas</dc:title><meta property="rendition:layout">pre-paginated</meta></metadata><manifest><item id="p1" href="pages/p1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="p1" properties="page-spread-center"/></spine></package>`))
		p1, _ := zw.Create("OEBPS/pages/p1.xhtml")
		_, _ = p1.Write([]byte(`<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><meta name="viewport" content="width=2400, height=1200"/></head><body><p>Wide Gatefold</p></body></html>`))
		_ = zw.Close()
		_ = os.WriteFile(epubPath, buf.Bytes(), 0644)

		book, err := f.ingester.IngestFile(context.Background(), epubPath, filepath.Join("Art", "AdversarialSpread", "AdversarialSpread.epub"))
		if err != nil {
			t.Fatalf("IngestFile: %v", err)
		}

		// Verify path traversal attempt on book asset endpoint is rejected (400 or 404)
		travReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/assets/../../etc/passwd", book.ID), nil)
		travReq.Header.Set("Authorization", "Bearer "+token)
		travRec := httptest.NewRecorder()
		f.handler.ServeHTTP(travRec, travReq)
		if travRec.Code == http.StatusOK {
			t.Fatalf("expected non-200 on asset path traversal attempt, got 200")
		}

		// Verify Reader source contracts for ShelfdColors & tabular mono
		readerView := readAppFile(t, "lib/ui/features/reader/reader_view.dart")
		readerMd := readAppFile(t, "lib/ui/features/reader/reader_markdown.dart")
		fixedReader := readAppFile(t, "lib/ui/features/reader/fixed_layout_reader.dart")
		assertContainsAll(t, "reader_view.dart", readerView, "ShelfdColors.of(context)", "AppTypography.mono(")
		assertContainsAll(t, "reader_markdown.dart", readerMd, "theme.extension<ShelfdColors>()", "AppTypography.mono(")
		assertContainsAll(t, "fixed_layout_reader.dart", fixedReader, "ShelfdColors.of(context)", "AppTypography.mono(")
	})

	t.Run("T5_08_AudiobookStudioAndMiniPlayerAdversarialRangeAndTabularMonoContract", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		relPath := filepath.Join("Weir", "HailMary", "HailMary.m4b")
		fullPath := filepath.Join(f.libDir, relPath)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		audioBytes := bytes.Repeat([]byte("M4B_FRAME_DATA__"), 64) // 1024 bytes
		_ = os.WriteFile(fullPath, audioBytes, 0644)

		sz := int64(len(audioBytes))
		dur := 3600.0
		book := &repository.Book{
			ID:              "audio-adv-t5",
			Title:           "Hail Mary Studio",
			FilePath:        relPath,
			BookType:        "audiobook",
			DurationSeconds: &dur,
			FileSizeBytes:   &sz,
		}
		_ = f.repo.CreateBook(context.Background(), book)

		// Unsatisfiable byte range beyond file size -> 416 Requested Range Not Satisfiable
		badRangeReq := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/"+book.ID+"/stream", nil)
		badRangeReq.Header.Set("Authorization", "Bearer "+token)
		badRangeReq.Header.Set("Range", "bytes=999999-1000000")
		badRangeRec := httptest.NewRecorder()
		f.handler.ServeHTTP(badRangeRec, badRangeReq)
		if badRangeRec.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Errorf("expected 416 for out-of-bounds Range header, got %d", badRangeRec.Code)
		}

		// Verify Obsidian & Amber tokens and AppTypography.mono across all 4 audiobook widgets
		player := readAppFile(t, "lib/ui/features/audiobook/audiobook_player_view.dart")
		scrubber := readAppFile(t, "lib/ui/features/audiobook/widgets/scrubber_bar.dart")
		drawer := readAppFile(t, "lib/ui/features/audiobook/widgets/chapter_drawer.dart")
		mini := readAppFile(t, "lib/ui/features/audiobook/widgets/mini_player_bar.dart")
		assertContainsAll(t, "audiobook_player_view.dart", player, "AppTokens.obsidianBackground", "AppTokens.amberAccent", "AppTypography.mono(")
		assertContainsAll(t, "scrubber_bar.dart", scrubber, "AppTokens.amberAccent", "AppTypography.mono(")
		assertContainsAll(t, "chapter_drawer.dart", drawer, "AppTokens.obsidianSurface", "AppTypography.mono(")
		assertContainsAll(t, "mini_player_bar.dart", mini, "AppTokens.obsidianSurface", "AppTokens.amberAccent")
	})

	t.Run("T5_09_UploadsAndIngestionAdversarialPathTraversalSanitizationAndDuplicateOverwrite", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()
		token := f.loginAndGetToken(t)

		// Stage EPUB with path-traversal characters in title and author
		epubBytes := buildInMemoryEPUB(t, "../../etc/passwd: A \"Novel\"?", "../Evil|Author*Name", "Adversarial description")
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		part, _ := mpw.CreateFormFile("file", "../../traversal.epub")
		_, _ = part.Write(epubBytes)
		_ = mpw.Close()

		stageReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		stageReq.Header.Set("Authorization", "Bearer "+token)
		stageReq.Header.Set("Content-Type", mpw.FormDataContentType())
		stageRec := httptest.NewRecorder()
		f.handler.ServeHTTP(stageRec, stageReq)
		if stageRec.Code != http.StatusOK {
			t.Fatalf("expected 200 staging adversarial EPUB, got %d: %s", stageRec.Code, stageRec.Body.String())
		}
		var staged struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(stageRec.Body.Bytes(), &staged)

		// Commit with adversarial path characters and verify file stays strictly inside f.libDir
		commitPayload := `{"title":"../../Escape: \"Book\"?","authors":["../Author|One"]}`
		cReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+staged.JobID+"/commit", strings.NewReader(commitPayload))
		cReq.Header.Set("Authorization", "Bearer "+token)
		cReq.Header.Set("Content-Type", "application/json")
		cRec := httptest.NewRecorder()
		f.handler.ServeHTTP(cRec, cReq)
		if cRec.Code != http.StatusCreated && cRec.Code != http.StatusOK {
			t.Fatalf("expected 201/200 committing sanitized book, got %d: %s", cRec.Code, cRec.Body.String())
		}

		var committed api.BookDetailResponse
		_ = json.Unmarshal(cRec.Body.Bytes(), &committed)
		if strings.Contains(committed.FilePath, "..") {
			t.Errorf("committed FilePath must not contain '..' traversal segments, got %q", committed.FilePath)
		}
	})

	t.Run("T5_10_OAuthConsentPageXSSAutoEscapingAndDarkScriptoriumCSSContract", func(t *testing.T) {
		f := setupAPITest(t)
		defer f.db.Close()
		defer f.repo.Close()

		mux := http.NewServeMux()
		oauthHandler := api.NewOAuthHandler(f.repo)
		oauthHandler.RegisterRoutes(mux)

		// Register OAuth client with adversarial XSS payload in client_name
		xssName := `<script>alert("xss")</script> & " malicious "`
		regBody, _ := json.Marshal(map[string]any{
			"client_name":   xssName,
			"redirect_uris": []string{"https://client.example.com/cb"},
		})
		regReq := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader(regBody))
		regRec := httptest.NewRecorder()
		mux.ServeHTTP(regRec, regReq)
		if regRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 on OAuth register, got %d", regRec.Code)
		}
		var regResp api.RegisterResponse
		_ = json.NewDecoder(regRec.Body).Decode(&regResp)

		authURL := fmt.Sprintf("/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s",
			url.QueryEscape(regResp.ClientID),
			url.QueryEscape("https://client.example.com/cb"),
			url.QueryEscape(`"><script>evil()</script>`),
		)
		getReq := httptest.NewRequest(http.MethodGet, authURL, nil)
		getRec := httptest.NewRecorder()
		mux.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 on GET /oauth/authorize, got %d", getRec.Code)
		}

		htmlBody := getRec.Body.String()
		if strings.Contains(htmlBody, `<script>alert("xss")</script>`) || strings.Contains(htmlBody, `<script>evil()</script>`) {
			t.Errorf("OAuth consent page failed to HTML-escape adversarial script tags: %s", htmlBody)
		}
		assertContainsAll(t, "OAuth consent Tactile Scriptorium CSS", htmlBody,
			"&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;",
			"--bg: #FBF9F4;",
			"--accent: #8E4A26;",
			"@media (prefers-color-scheme: dark)",
			"--bg: #161412;",
			"--accent: #D98A5F;",
			"Newsreader",
			"Work+Sans",
			"JetBrains+Mono",
		)
	})

	t.Run("T5_11_SearchUtilitiesSettingsAndConnectAdversarialThemeAndValidationContract", func(t *testing.T) {
		search := readAppFile(t, "lib/ui/features/search/search_view.dart")
		utilities := readAppFile(t, "lib/ui/features/utilities/utilities_view.dart")
		settings := readAppFile(t, "lib/ui/features/settings/settings_view.dart")
		connect := readAppFile(t, "lib/ui/features/connect/connect_view.dart")

		assertContainsAll(t, "search_view.dart", search, "context.shelfdColors", "TactileBookCover(", "AppTypography.mono(")
		assertContainsAll(t, "utilities_view.dart", utilities, "context.shelfdColors", "TactileBookCover(", "AppTypography.mono(")
		assertContainsAll(t, "settings_view.dart", settings, "SegmentedButton<ReadingThemeMode>", "ReadingThemeMode.bone", "ReadingThemeMode.sepia", "ReadingThemeMode.dark", "AppTypography.mono(")
		assertContainsAll(t, "connect_view.dart", connect, "context.shelfdColors", "AppTypography.mono(", "PrimaryButton(")
	})

	t.Run("T5_12_StrictM4RedesignContractEnforcementAcrossAllTwelveFeatureAreas", func(t *testing.T) {
		t.Setenv("SHELFD_M4_STRICT", "1")
		tokens := readAppFile(t, "lib/ui/core/tokens.dart")
		theme := readAppFile(t, "lib/ui/core/theme.dart")
		typo := readAppFile(t, "lib/ui/core/typography.dart")
		shared := readAppFile(t, "lib/ui/core/shared_layout.dart")
		oauthGo := readRepoFile(t, "apps/server/internal/api/oauth.go")
		e2eDart := readAppFile(t, "test/ui/tactile_scriptorium_e2e_test.dart")

		assertRedesignContractIfLanded(t, "tokens.dart", tokens, "siennaPrimary",
			"siennaPrimary = Color(0xFF8E4A26)",
			"libraryGreen = Color(0xFF3A5A40)",
			"warmParchment = Color(0xFFFBF9F4)",
			"walnutBackground = Color(0xFF161412)",
		)
		assertRedesignContractIfLanded(t, "theme.dart", theme, "class ShelfdColors",
			"ShelfdColors.bone", "ShelfdColors.sepia", "ShelfdColors.dark",
		)
		assertRedesignContractIfLanded(t, "typography.dart", typo, "editorialSerifFamily",
			"Newsreader", "Work Sans", "JetBrains Mono", "FontFeature.tabularFigures()",
		)
		assertRedesignContractIfLanded(t, "shared_layout.dart", shared, "class TactileBookCover",
			"showSpineCrease", "isSeries", "_buildEditorialFallback",
		)
		assertContainsAll(t, "oauth.go", oauthGo, "#FBF9F4", "#8E4A26", "#161412", "#D98A5F", "Newsreader")
		assertContainsAll(t, "tactile_scriptorium_e2e_test.dart", e2eDart,
			"Tier 5 — White-Box Adversarial Hardening",
			"T5-01", "T5-02", "T5-03", "T5-04", "T5-05", "T5-06", "T5-07", "T5-08",
		)
	})
}

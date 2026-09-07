# EPUB Metadata Extraction Manual

This technical manual instructs the AI on extracting Dublin Core metadata, series tags, and plain chapter text from EPUB packages.

## Protocols

### 1. Package Unpacking
* Locate root package file via `META-INF/container.xml`.
* Parse XML namespace and manifest items.

### 2. Dublin Core Mapping
* `<dc:title>` -> Book Title
* `<dc:creator>` -> Author(s) (supports multiple contributors)
* `<dc:subject>` -> Genre(s) / Tag(s)
* `<dc:description>` -> Book Synopsis
* `<dc:publisher>` -> Publisher
* `<dc:language>` -> Language code

### 3. Series Extraction Priority
1. **EPUB 3 Standard**:
   * `<meta property="belongs-to-collection" id="c01">Series Name</meta>`
   * `<meta refines="#c01" property="group-position">1.0</meta>`
2. **Calibre / EPUB 2 Standard**:
   * `<meta name="calibre:series" content="Series Name"/>`
   * `<meta name="calibre:series_index" content="1.0"/>`

### 4. Chapter Text Extraction
* Walk the spine items in reading order.
* Strip XML/HTML tags and inline styling.
* Preserve paragraph breaks (`\n\n`) for clean reader display and chapter summarization.

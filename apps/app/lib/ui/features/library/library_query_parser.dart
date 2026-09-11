import '../../../data/models/book.dart';

/// Structured result of parsing a library search/filter query.
class ParsedLibraryQuery {
  final String raw;
  final String titleQuery;
  final String? authorQuery;
  final String? genreQuery;
  final String? topicQuery;

  const ParsedLibraryQuery({
    required this.raw,
    required this.titleQuery,
    this.authorQuery,
    this.genreQuery,
    this.topicQuery,
  });

  static final RegExp _tokenRegex = RegExp(
    r'\b(author|genre|topic):\s*(?:"([^"]*)"|(\S+))',
    caseSensitive: false,
  );

  static final RegExp _whitespaceRegex = RegExp(r'\s+');

  /// Parses a raw user query string into structured filter fields.
  static ParsedLibraryQuery parse(String raw) {
    final trimmed = raw.trim();
    if (trimmed.isEmpty) {
      return const ParsedLibraryQuery(raw: '', titleQuery: '');
    }

    String? author;
    String? genre;
    String? topic;

    final matches = _tokenRegex.allMatches(trimmed);
    for (final m in matches) {
      final tag = m.group(1)?.toLowerCase();
      final quotedVal = m.group(2);
      final unquotedVal = m.group(3);
      final val = (quotedVal ?? unquotedVal ?? '').trim();

      if (val.isNotEmpty) {
        switch (tag) {
          case 'author':
            author = val;
            break;
          case 'genre':
            genre = val;
            break;
          case 'topic':
            topic = val;
            break;
        }
      }
    }

    final withoutTokens = trimmed.replaceAll(_tokenRegex, ' ');
    final cleanTitle = withoutTokens.replaceAll(_whitespaceRegex, ' ').trim();

    return ParsedLibraryQuery(
      raw: raw,
      titleQuery: cleanTitle,
      authorQuery: author,
      genreQuery: genre,
      topicQuery: topic,
    );
  }

  bool get hasFilter =>
      titleQuery.isNotEmpty ||
      (authorQuery != null && authorQuery!.isNotEmpty) ||
      (genreQuery != null && genreQuery!.isNotEmpty) ||
      (topicQuery != null && topicQuery!.isNotEmpty);

  /// Evaluates whether a [book] satisfies all parsed filter conditions.
  bool matchesBook(Book book) {
    if (!hasFilter) return true;

    // 1. Title matching
    if (titleQuery.isNotEmpty) {
      final tQuery = titleQuery.toLowerCase();
      final matchesTitle = book.title.toLowerCase().contains(tQuery);
      final matchesSynopsis = book.synopsis.toLowerCase().contains(tQuery);
      if (!matchesTitle && !matchesSynopsis) {
        return false;
      }
    }

    // 2. Author matching
    if (authorQuery != null && authorQuery!.isNotEmpty) {
      final aQuery = authorQuery!.toLowerCase().trim();
      final queryTokens = aQuery.split(_whitespaceRegex).where((t) => t.isNotEmpty).toList();

      final authorNames = book.authors.map((a) => a.name.toLowerCase()).toList();
      if (book.authorDisplay.isNotEmpty) {
        authorNames.add(book.authorDisplay.toLowerCase());
      }

      final matchesAuthor = authorNames.any((name) {
        // Direct substring match either way
        if (name.contains(aQuery) || aQuery.contains(name)) return true;
        // Token match: every token in query appears in author name
        if (queryTokens.isNotEmpty && queryTokens.every((tok) => name.contains(tok))) {
          return true;
        }
        return false;
      });

      if (!matchesAuthor) return false;
    }

    // 3. Genre matching
    if (genreQuery != null && genreQuery!.isNotEmpty) {
      final gQuery = genreQuery!.toLowerCase().trim();
      final matchesGenre = book.genres.any((g) => g.name.toLowerCase().contains(gQuery));
      if (!matchesGenre) return false;
    }

    // 4. Topic matching
    if (topicQuery != null && topicQuery!.isNotEmpty) {
      final topQuery = topicQuery!.toLowerCase().trim();
      final matchesTopic = book.topics.any((t) => t.name.toLowerCase().contains(topQuery));
      if (!matchesTopic) return false;
    }

    return true;
  }

  /// Detects whether the cursor in [text] is currently inside an autocomplete trigger token
  /// like `author:`, `genre:`, or `topic:`.
  /// Returns a record with `tag` and `prefix` if active, or null.
  static ({String tag, String prefix, int tokenStart, int tokenEnd})? detectAutocompletePrefix(
    String text,
    int cursorPosition,
  ) {
    if (text.isEmpty || cursorPosition < 0 || cursorPosition > text.length) {
      return null;
    }

    final sub = text.substring(0, cursorPosition);
    final triggerRegex = RegExp(
      r'(author|genre|topic):\s*(?:"([^"]*)"?|([^\s"]*))$',
      caseSensitive: false,
    );
    final match = triggerRegex.firstMatch(sub);

    if (match != null) {
      final tag = match.group(1)!.toLowerCase();
      final quoted = match.group(2);
      final unquoted = match.group(3);
      final prefix = quoted ?? unquoted ?? '';
      final tokenStart = match.start;
      final tokenEnd = cursorPosition;

      return (
        tag: tag,
        prefix: prefix,
        tokenStart: tokenStart,
        tokenEnd: tokenEnd,
      );
    }

    return null;
  }
}

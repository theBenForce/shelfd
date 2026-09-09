import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/highlight.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/reader/reader_markdown.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('parseReaderBlocks', () {
    test('parses headings correctly', () {
      final markdown = '# Main Heading\n\n## Subheading\n\n### Section\n\n#### Subsection';
      final blocks = parseReaderBlocks(markdown);

      expect(blocks.length, 4);
      expect(blocks[0].type, ReaderBlockType.h1);
      expect(blocks[0].text, 'Main Heading');

      expect(blocks[1].type, ReaderBlockType.h2);
      expect(blocks[1].text, 'Subheading');

      expect(blocks[2].type, ReaderBlockType.h3);
      expect(blocks[2].text, 'Section');

      expect(blocks[3].type, ReaderBlockType.h4);
      expect(blocks[3].text, 'Subsection');
    });

    test('parses blockquotes and dividers', () {
      final markdown = '> A quote from history.\n> Second quote line.\n\n---\n\nNormal paragraph.';
      final blocks = parseReaderBlocks(markdown);

      expect(blocks.length, 3);
      expect(blocks[0].type, ReaderBlockType.blockquote);
      expect(blocks[0].text, 'A quote from history.\nSecond quote line.');

      expect(blocks[1].type, ReaderBlockType.divider);

      expect(blocks[2].type, ReaderBlockType.paragraph);
      expect(blocks[2].text, 'Normal paragraph.');
    });
  });

  group('parseInlineSpans', () {
    test('parses inline bold, italic, bold-italic, and footnotes', () {
      const text = 'Here is a **bold** word, an *italic* thought, ***both*** together, and a footnote[5].';
      const baseStyle = TextStyle(fontSize: 16, color: Colors.black);
      const accent = Colors.red;

      final spans = parseInlineSpans(text, baseStyle, accent);

      // Verify non-empty spans produced
      expect(spans.isNotEmpty, true);

      // Verify footnote span is a WidgetSpan
      final widgetSpans = spans.whereType<WidgetSpan>().toList();
      expect(widgetSpans.length, 1);

      // Verify text spans
      final textSpans = spans.whereType<TextSpan>().toList();
      expect(textSpans.any((s) => s.text == 'bold' && s.style?.fontWeight == FontWeight.bold), true);
      expect(textSpans.any((s) => s.text == 'italic' && s.style?.fontStyle == FontStyle.italic), true);
      expect(
        textSpans.any(
          (s) => s.text == 'both' && s.style?.fontWeight == FontWeight.bold && s.style?.fontStyle == FontStyle.italic,
        ),
        true,
      );
    });
  });

  group('buildReaderBlockWidget', () {
    testWidgets('renders H2 and paragraph with footnote widget', (tester) async {
      const h2Block = ReaderBlock(type: ReaderBlockType.h2, text: 'The reign of the Dixiecrats');
      const pBlock = ReaderBlock(
        type: ReaderBlockType.paragraph,
        text: 'Red-blooded men know what I mean.”[5]',
      );

      final settings = const ReaderSettings();
      final theme = AppTheme.buildTheme(ReadingThemeMode.bone);

      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          home: Scaffold(
            body: Column(
              children: [
                buildReaderBlockWidget(block: h2Block, settings: settings, theme: theme),
                buildReaderBlockWidget(block: pBlock, settings: settings, theme: theme),
              ],
            ),
          ),
        ),
      );

      expect(find.text('The reign of the Dixiecrats'), findsOneWidget);
      expect(find.textContaining('Red-blooded men know what I mean.”'), findsOneWidget);
      expect(find.text('5'), findsOneWidget);
    });

    testWidgets('renders highlights with Kindle color and triggers onHighlightTap', (tester) async {
      const pBlock = ReaderBlock(
        type: ReaderBlockType.paragraph,
        text: 'Call me Ishmael. Some years ago never mind how long precisely.',
      );

      final highlight = Highlight(
        id: 'hl-123',
        bookId: 'b-1',
        selectedText: 'Call me Ishmael.',
        color: 'yellow',
      );

      Highlight? tappedHighlight;
      final settings = const ReaderSettings();
      final theme = AppTheme.buildTheme(ReadingThemeMode.bone);

      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          home: Scaffold(
            body: buildReaderBlockWidget(
              block: pBlock,
              settings: settings,
              theme: theme,
              highlights: [highlight],
              onHighlightTap: (hl) => tappedHighlight = hl,
            ),
          ),
        ),
      );

      // Verify text renders
      expect(find.textContaining('Call me Ishmael.'), findsOneWidget);

      // Tap the highlighted text at the start of the widget
      final textFinder = find.textContaining('Call me Ishmael.');
      final topLeft = tester.getTopLeft(textFinder);
      await tester.tapAt(topLeft + const Offset(15, 8));
      await tester.pump();

      expect(tappedHighlight, isNotNull);
      expect(tappedHighlight!.id, 'hl-123');
      expect(tappedHighlight!.highlightColor, KindleHighlightColor.yellow);
    });

    testWidgets('renders multi-paragraph highlight across consecutive blocks', (tester) async {
      const pBlock1 = ReaderBlock(
        type: ReaderBlockType.paragraph,
        text: 'The first paragraph ends here with deep reflection.',
        startOffset: 0,
        endOffset: 52,
        paragraphIndex: 1,
      );
      const pBlock2 = ReaderBlock(
        type: ReaderBlockType.paragraph,
        text: 'The second paragraph begins with a fresh observation.',
        startOffset: 54,
        endOffset: 107,
        paragraphIndex: 2,
      );

      final multiParaHighlight = Highlight(
        id: 'hl-multi',
        bookId: 'b-1',
        selectedText: 'deep reflection.\n\nThe second paragraph begins',
        color: 'blue',
        startOffset: 36,
        endOffset: 84,
        startParagraph: 1,
        endParagraph: 2,
      );

      Highlight? tappedHighlight;
      final settings = const ReaderSettings();
      final theme = AppTheme.buildTheme(ReadingThemeMode.bone);

      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          home: Scaffold(
            body: Column(
              children: [
                buildReaderBlockWidget(
                  block: pBlock1,
                  settings: settings,
                  theme: theme,
                  highlights: [multiParaHighlight],
                  onHighlightTap: (hl) => tappedHighlight = hl,
                ),
                buildReaderBlockWidget(
                  block: pBlock2,
                  settings: settings,
                  theme: theme,
                  highlights: [multiParaHighlight],
                  onHighlightTap: (hl) => tappedHighlight = hl,
                ),
              ],
            ),
          ),
        ),
      );

      // Verify text from both blocks renders
      expect(find.textContaining('deep reflection.'), findsOneWidget);
      expect(find.textContaining('The second paragraph begins'), findsOneWidget);

      // Tapping second paragraph highlighted text triggers callback for the unified highlight
      final p2Finder = find.textContaining('The second paragraph begins');
      final p2TopLeft = tester.getTopLeft(p2Finder);
      await tester.tapAt(p2TopLeft + const Offset(20, 8));
      await tester.pump();

      expect(tappedHighlight, isNotNull);
      expect(tappedHighlight!.id, 'hl-multi');
      expect(tappedHighlight!.highlightColor, KindleHighlightColor.blue);
      expect(tappedHighlight!.startParagraph, 1);
      expect(tappedHighlight!.endParagraph, 2);
    });
  });

  group('findHighlightRange', () {
    const content =
        'The first paragraph ends here with deep reflection.\n\nThe second paragraph begins with a fresh observation.';
    final blocks = parseReaderBlocks(content);

    test('finds range within a single paragraph', () {
      final range = findHighlightRange(
        blocks: blocks,
        fullContent: content,
        selectedText: 'deep reflection.',
      );

      expect(range, isNotNull);
      expect(range!.startOffset, 35);
      expect(range.endOffset, 51);
      expect(range.startParagraph, 1);
      expect(range.endParagraph, 1);
    });

    test('finds unified range across two paragraphs', () {
      // User selects end of paragraph 1 and start of paragraph 2
      final range = findHighlightRange(
        blocks: blocks,
        fullContent: content,
        selectedText: 'deep reflection.\n\nThe second paragraph begins',
      );

      expect(range, isNotNull);
      expect(range!.startOffset, 35);
      expect(range.endOffset, 80);
      expect(range.startParagraph, 1);
      expect(range.endParagraph, 2);
    });

    test('finds unified range when selection separated by single newline', () {
      final range = findHighlightRange(
        blocks: blocks,
        fullContent: content,
        selectedText: 'deep reflection.\nThe second paragraph begins',
      );

      expect(range, isNotNull);
      expect(range!.startOffset, 35);
      expect(range.endOffset, 80);
      expect(range.startParagraph, 1);
      expect(range.endParagraph, 2);
    });

    test('returns null for empty or whitespace selection', () {
      final range = findHighlightRange(
        blocks: blocks,
        fullContent: content,
        selectedText: '   \n  ',
      );
      expect(range, isNull);
    });
  });
}
